package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Core.LogLevel != "info" {
		t.Errorf("expected info, got %s", cfg.Core.LogLevel)
	}
	if cfg.Server.Port != 8472 {
		t.Errorf("expected 8472, got %d", cfg.Server.Port)
	}
	if cfg.Model.DefaultProvider != "ollama" {
		t.Errorf("expected ollama, got %s", cfg.Model.DefaultProvider)
	}
	if cfg.UI.Theme != "system" {
		t.Errorf("expected system, got %s", cfg.UI.Theme)
	}
}

func TestLoadReturnsDefaultsOnFirstRun(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Core.LogLevel != "info" {
		t.Errorf("expected info, got %s", cfg.Core.LogLevel)
	}
	configPath := filepath.Join(tmpDir, ".config", "nala", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestEnvVarOverrides(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	os.Setenv("NALA_CORE_LOG_LEVEL", "debug")
	os.Setenv("NALA_SERVER_PORT", "9090")
	os.Setenv("NALA_UI_THEME", "dark")
	defer func() {
		os.Unsetenv("NALA_CORE_LOG_LEVEL")
		os.Unsetenv("NALA_SERVER_PORT")
		os.Unsetenv("NALA_UI_THEME")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Core.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.Core.LogLevel)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected 9090, got %d", cfg.Server.Port)
	}
	if cfg.UI.Theme != "dark" {
		t.Errorf("expected dark, got %s", cfg.UI.Theme)
	}
}

func TestEnvVarBoolOverride(t *testing.T) {
	os.Setenv("NALA_SERVER_ENABLED", "false")
	defer os.Unsetenv("NALA_SERVER_ENABLED")

	cfg := DefaultConfig()
	applyEnvOverrides(&cfg)
	if cfg.Server.Enabled {
		t.Error("expected server.enabled to be false")
	}
}

func TestEnvVarArrayOverride(t *testing.T) {
	os.Setenv("NALA_TOOLS_ALLOWED_LANGUAGES", "python,ruby")
	defer os.Unsetenv("NALA_TOOLS_ALLOWED_LANGUAGES")

	cfg := DefaultConfig()
	applyEnvOverrides(&cfg)
	if len(cfg.Tools.AllowedLanguages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(cfg.Tools.AllowedLanguages))
	}
	if cfg.Tools.AllowedLanguages[0] != "python" {
		t.Errorf("expected python, got %s", cfg.Tools.AllowedLanguages[0])
	}
}

func TestValidateInvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.LogLevel = "invalid"
	if err := validate(&cfg); err != nil {
		t.Fatalf("validate() should not error on invalid log level (silent fix): %v", err)
	}
	if cfg.Core.LogLevel != "info" {
		t.Errorf("expected fallback to info, got %s", cfg.Core.LogLevel)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 80
	if err := validate(&cfg); err == nil {
		t.Error("expected error for port 80")
	}
}

func TestValidateInvalidTheme(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UI.Theme = "neon"
	if err := validate(&cfg); err == nil {
		t.Error("expected error for invalid theme")
	}
}

func TestValidateInvalidLanguage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UI.Language = "fr"
	if err := validate(&cfg); err == nil {
		t.Error("expected error for invalid language")
	}
}

func TestGetReturnsCurrent(t *testing.T) {
	saved := current
	defer func() { current = saved }()

	current = DefaultConfig()
	got := Get()
	if got.Core.LogLevel != "info" {
		t.Errorf("expected info, got %s", got.Core.LogLevel)
	}
}
