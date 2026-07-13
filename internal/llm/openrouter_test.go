package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/juuso/auto-command/internal/config"
)

// newTestClient returns a Client whose endpoint points at srv.
func newTestClient(srv *httptest.Server) *Client {
	c := New(&config.Config{
		APIKey:         "test-key",
		Model:          "openai/gpt-4o-mini",
		MaxSuggestions: 3,
	})
	c.endpoint = srv.URL
	return c
}

// chatCompletionJSON wraps assistant content in a chat completions envelope.
func chatCompletionJSON(t *testing.T, content string) string {
	t.Helper()
	resp := chatResponse{}
	resp.Choices = make([]struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}, 1)
	resp.Choices[0].Message.Content = content
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return string(b)
}

func TestSuggest_CleanJSONSchemaResponse(t *testing.T) {
	content := `{"suggestions":[{"command":"ls -la","explanation":"List all files"},{"command":"ls","explanation":"List files"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("HTTP-Referer"); got == "" {
			t.Error("HTTP-Referer header missing")
		}
		if got := r.Header.Get("X-Title"); got == "" {
			t.Error("X-Title header missing")
		}

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_schema" {
			t.Errorf("response_format not set to json_schema: %+v", req.ResponseFormat)
		}
		if req.Model != "openai/gpt-4o-mini" {
			t.Errorf("model = %q", req.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, chatCompletionJSON(t, content))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Suggest(context.Background(), "list files")
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d suggestions, want 2", len(got))
	}
	if got[0].Command != "ls -la" || got[0].Explanation != "List all files" {
		t.Errorf("first suggestion = %+v", got[0])
	}
}

func TestSuggest_FencedJSONFallback(t *testing.T) {
	// A model wraps its JSON in a markdown fence and adds prose around it.
	content := "Sure! Here you go:\n\n```json\n" +
		`{"suggestions":[{"command":"echo \"hi there\"","explanation":"Print a line\nwith a newline"}]}` +
		"\n```\nHope that helps!"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, chatCompletionJSON(t, content))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Suggest(context.Background(), "print hi")
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(got))
	}
	// Quotes and newlines must survive parsing intact.
	if got[0].Command != `echo "hi there"` {
		t.Errorf("command = %q, want echo \"hi there\"", got[0].Command)
	}
	if got[0].Explanation != "Print a line\nwith a newline" {
		t.Errorf("explanation = %q", got[0].Explanation)
	}
}

func TestSuggest_EmptySuggestions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, chatCompletionJSON(t, `{"suggestions":[]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Suggest(context.Background(), "do nothing")
	if !errors.Is(err, ErrNoSuggestions) {
		t.Fatalf("err = %v, want ErrNoSuggestions", err)
	}
}

func TestSuggest_HTTPErrorStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "401 unauthorized",
			status: http.StatusUnauthorized,
			body:   `{"error":{"message":"No auth credentials found","code":401}}`,
			want:   "No auth credentials found",
		},
		{
			name:   "429 rate limited",
			status: http.StatusTooManyRequests,
			body:   `{"error":{"message":"Rate limit exceeded"}}`,
			want:   "Rate limit exceeded",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			_, err := newTestClient(srv).Suggest(context.Background(), "q")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if !strings.Contains(apiErr.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", apiErr.Error(), tc.want)
			}
		})
	}
}
