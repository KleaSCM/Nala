package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeFieldsMap(t *testing.T) {
	safe := SafeFields()
	if !safe["ui.theme"] {
		t.Error("expected ui.theme to be safe")
	}
	if !safe["core.log_level"] {
		t.Error("expected core.log_level to be safe")
	}
}

func TestUnsafeFieldsMap(t *testing.T) {
	unsafe := UnsafeFields()
	if !unsafe["core.data_dir"] {
		t.Error("expected core.data_dir to be unsafe")
	}
	if !unsafe["server.port"] {
		t.Error("expected server.port to be unsafe")
	}
}

func TestOnReloadCallback(t *testing.T) {
	called := false
	OnReload(func(old, new Config) {
		called = true
	}, true)

	notifyReload(DefaultConfig(), DefaultConfig())

	if !called {
		t.Error("expected reload callback to be called")
	}
}

func TestGetAfterSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.LogLevel = "debug"
	setCurrent(cfg)

	got := Get()
	if got.Core.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", got.Core.LogLevel)
	}

	// reset
	setCurrent(DefaultConfig())
}

func TestConcurrentGetSet(t *testing.T) {
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			Get()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			cfg := DefaultConfig()
			cfg.Core.LogLevel = "debug"
			setCurrent(cfg)
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

func TestStartStopWatcherNoPanic(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Load to create config file
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	err = StartWatcher()
	if err != nil {
		t.Logf("watcher start error (may be expected in CI): %v", err)
		return
	}

	StopWatcher()
}

func TestConfigPathIsCorrect(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath() error: %v", err)
	}
	expected := filepath.Join(tmpDir, ".config", "nala", "config.toml")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
