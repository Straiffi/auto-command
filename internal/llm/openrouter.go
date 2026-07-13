// Package llm calls the OpenRouter chat completions endpoint and turns a
// natural-language query into discrete command suggestions. It parses the
// model's structured JSON response, falling back to a lenient extractor when a
// model ignores the requested format.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juuso/auto-command/internal/config"
	"github.com/juuso/auto-command/internal/envctx"
	"github.com/juuso/auto-command/internal/prompt"
)

// DefaultEndpoint is OpenRouter's OpenAI-compatible chat completions endpoint.
// It is a variable (not a hardcoded literal at the call site) so it is easy to
// point at a mock server in tests or change if the provider path moves.
const DefaultEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// requestTimeout bounds a single suggestion request end to end.
const requestTimeout = 30 * time.Second

// Referer and title are OpenRouter's recommended attribution headers.
const (
	httpReferer = "https://github.com/juuso/auto-command"
	xTitle      = "auto-command"
)

// ErrNoSuggestions is returned when the model produces a well-formed response
// whose suggestions array is empty. Callers can show a friendly message.
var ErrNoSuggestions = errors.New("model returned no suggestions")

// Suggestion is a single command proposal.
type Suggestion struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

// APIError is returned for non-2xx responses. It carries the HTTP status and any
// provider-supplied error message.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("openrouter: %s: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("openrouter: %s", e.Status)
}

// Client calls OpenRouter. The zero value is not usable; use New.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	endpoint   string
}

// New returns a Client bound to the given configuration.
func New(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{},
		endpoint:   DefaultEndpoint,
	}
}

// Suggest turns a query into command suggestions. It applies a request timeout,
// derived from the parent context, and returns ErrNoSuggestions when the model
// returns an empty array.
func (c *Client) Suggest(ctx context.Context, query string) ([]Suggestion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	msgs := prompt.Build(envctx.Detect(), query, c.cfg.MaxSuggestions)
	body, err := json.Marshal(buildRequest(c.cfg.Model, msgs, c.cfg.MaxSuggestions))
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("HTTP-Referer", httpReferer)
	req.Header.Set("X-Title", xTitle)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting OpenRouter (check network connectivity): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    extractProviderError(respBody),
		}
	}

	content, err := decodeContent(respBody)
	if err != nil {
		return nil, err
	}

	suggestions, err := parseSuggestions(content)
	if err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, ErrNoSuggestions
	}
	return suggestions, nil
}

// --- request/response payloads ---

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// buildRequest assembles the chat completions payload, including a json_schema
// response_format matching the suggestions shape bounded by maxSuggestions.
func buildRequest(model string, msgs []prompt.Message, maxSuggestions int) chatRequest {
	cm := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		cm[i] = chatMessage{Role: m.Role, Content: m.Content}
	}
	return chatRequest{
		Model:          model,
		Messages:       cm,
		ResponseFormat: suggestionsResponseFormat(maxSuggestions),
	}
}

func suggestionsResponseFormat(maxSuggestions int) *responseFormat {
	if maxSuggestions < 1 {
		maxSuggestions = 1
	}
	itemSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":     map[string]any{"type": "string"},
			"explanation": map[string]any{"type": "string"},
		},
		"required":             []string{"command", "explanation"},
		"additionalProperties": false,
	}
	return &responseFormat{
		Type: "json_schema",
		JSONSchema: jsonSchema{
			Name:   "command_suggestions",
			Strict: true,
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"suggestions": map[string]any{
						"type":  "array",
						"items": itemSchema,
					},
				},
				"required":             []string{"suggestions"},
				"additionalProperties": false,
			},
		},
	}
}

// --- parsing ---

// decodeContent pulls the assistant message content out of the chat response
// envelope.
func decodeContent(body []byte) (string, error) {
	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decoding response envelope: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("openrouter: response contained no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

type suggestionsPayload struct {
	Suggestions []Suggestion `json:"suggestions"`
}

// parseSuggestions decodes the suggestions object from the model content. It
// first tries a strict unmarshal, then makes a single lenient attempt that
// strips markdown fences and extracts the first JSON object.
func parseSuggestions(content string) ([]Suggestion, error) {
	var payload suggestionsPayload
	if err := json.Unmarshal([]byte(content), &payload); err == nil {
		return payload.Suggestions, nil
	}

	extracted := extractJSONObject(content)
	if extracted == "" {
		return nil, fmt.Errorf("could not locate JSON object in model response: %q", truncate(content, 200))
	}
	if err := json.Unmarshal([]byte(extracted), &payload); err != nil {
		return nil, fmt.Errorf("parsing extracted JSON: %w", err)
	}
	return payload.Suggestions, nil
}

// extractJSONObject strips ```json / ``` fences and returns the substring from
// the first "{" to its matching "}", tracking string literals so braces inside
// quoted values do not confuse the scan. It returns "" when no balanced object
// is found.
func extractJSONObject(s string) string {
	s = stripFences(s)

	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// stripFences removes a leading ```json (or ```) fence and a trailing ``` fence
// if present.
func stripFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```")
	// Drop an optional language tag on the opening fence line.
	if nl := strings.IndexByte(t, '\n'); nl >= 0 {
		firstLine := strings.TrimSpace(t[:nl])
		if !strings.Contains(firstLine, "{") {
			t = t[nl+1:]
		}
	}
	if idx := strings.LastIndex(t, "```"); idx >= 0 {
		t = t[:idx]
	}
	return t
}

// extractProviderError pulls a human-readable message from an OpenRouter error
// body, tolerating both {"error":{"message":...}} and {"error":"..."} shapes.
func extractProviderError(body []byte) string {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Error) == 0 {
		return strings.TrimSpace(string(body))
	}

	var obj struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(envelope.Error, &obj); err == nil && obj.Message != "" {
		return obj.Message
	}

	var msg string
	if err := json.Unmarshal(envelope.Error, &msg); err == nil {
		return msg
	}
	return strings.TrimSpace(string(envelope.Error))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
