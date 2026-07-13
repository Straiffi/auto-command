package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// templateContents is the starter config written by EnsureTemplate. The api_key
// is left blank so no secret is ever committed to disk by default.
const templateContents = `# auto-command configuration

# OpenRouter API key. Prefer setting AUTO_COMMAND_API_KEY or OPENROUTER_API_KEY
# in your environment instead of storing it here.
api_key = ""

# OpenRouter model identifier.
model = "` + DefaultModel + `"

# Maximum number of command suggestions to request.
max_suggestions = 3
`

// EnsureTemplate creates the config directory (0700) and a template config file
// (0600) if the file does not already exist. It returns the config path and
// whether a new template file was created. It never writes a secret value.
func EnsureTemplate() (path string, created bool, err error) {
	path, err = Path()
	if err != nil {
		return "", false, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("checking config file %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(templateContents), 0o600); err != nil {
		return "", false, fmt.Errorf("writing config template %s: %w", path, err)
	}
	return path, true, nil
}
