package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearEnv unsets every environment variable Load consults so each test starts
// from a known state.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HOME", "XDG_CONFIG_HOME",
		"AUTO_COMMAND_API_KEY", "OPENROUTER_API_KEY", "AUTO_COMMAND_MODEL",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func writeConfig(t *testing.T, dir, contents string) {
	t.Helper()
	cfgDir := filepath.Join(dir, "auto-command")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestPathUsesXDGConfigHome(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	t.Setenv("HOME", "/home/user")

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/xdg", "auto-command", "config.toml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPathFallsBackToHomeConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/user")

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/home/user", ".config", "auto-command", "config.toml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadDefaultsWithEnvKeyNoFile(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AUTO_COMMAND_API_KEY", "env-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key", cfg.APIKey)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want default %q", cfg.Model, DefaultModel)
	}
	if cfg.MaxSuggestions != DefaultMaxSuggestions {
		t.Errorf("MaxSuggestions = %d, want %d", cfg.MaxSuggestions, DefaultMaxSuggestions)
	}
}

func TestLoadFileOverridesDefault(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeConfig(t, dir, `
api_key = "file-key"
model = "file/model"
max_suggestions = 7
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "file-key" {
		t.Errorf("APIKey = %q, want file-key", cfg.APIKey)
	}
	if cfg.Model != "file/model" {
		t.Errorf("Model = %q, want file/model", cfg.Model)
	}
	if cfg.MaxSuggestions != 7 {
		t.Errorf("MaxSuggestions = %d, want 7", cfg.MaxSuggestions)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeConfig(t, dir, `
api_key = "file-key"
model = "file/model"
`)
	t.Setenv("AUTO_COMMAND_API_KEY", "env-key")
	t.Setenv("AUTO_COMMAND_MODEL", "env/model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key (env over file)", cfg.APIKey)
	}
	if cfg.Model != "env/model" {
		t.Errorf("Model = %q, want env/model (env over file)", cfg.Model)
	}
}

func TestLoadAutoCommandKeyPreferredOverOpenRouter(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("AUTO_COMMAND_API_KEY", "auto-command-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "auto-command-key" {
		t.Errorf("APIKey = %q, want auto-command-key", cfg.APIKey)
	}
}

func TestLoadOpenRouterKeyFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIKey != "openrouter-key" {
		t.Errorf("APIKey = %q, want openrouter-key", cfg.APIKey)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want MissingError")
	}
	me, ok := err.(*MissingError)
	if !ok {
		t.Fatalf("error type = %T, want *MissingError", err)
	}
	if len(me.Fields) != 1 || me.Fields[0] != "api_key" {
		t.Errorf("MissingError.Fields = %v, want [api_key]", me.Fields)
	}
	// A Config is still returned with defaults applied.
	if cfg == nil || cfg.Model != DefaultModel {
		t.Errorf("expected defaulted Config alongside error, got %+v", cfg)
	}
}

func TestLoadMalformedTOML(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeConfig(t, dir, "this is = = not valid toml")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want parse error")
	}
	path := filepath.Join(dir, "auto-command", "config.toml")
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the config path %q", err.Error(), path)
	}
}

func TestEnsureTemplateCreatesFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, created, err := EnsureTemplate()
	if err != nil {
		t.Fatalf("EnsureTemplate() error = %v", err)
	}
	if !created {
		t.Error("created = false, want true on first call")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}

	// Second call must not report creation.
	_, created, err = EnsureTemplate()
	if err != nil {
		t.Fatalf("EnsureTemplate() second call error = %v", err)
	}
	if created {
		t.Error("created = true on second call, want false")
	}
}
