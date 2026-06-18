package engine

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if e.Config == nil {
		t.Error("expected config to be set")
	}
	if e.Logger == nil {
		t.Error("expected logger to be set")
	}
	if e.DB == nil {
		t.Error("expected db to be set")
	}
}

func TestEngineStartAndShutdown(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	e.Shutdown(5 * time.Second)
}

func TestEngineCreatesDataDir(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	dataDir := e.Config.Core.DataDir
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Error("data directory was not created")
	}
}

func TestEngineCreatesDBFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	dbPath := filepath.Join(e.Config.Core.DataDir, "nala.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestSignalTriggersShutdown(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	time.Sleep(100 * time.Millisecond)
}

func TestForceExitAfterTimeout(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	e, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	fatalCalled := false
	e.SetOnFatal(func(msg string) {
		fatalCalled = true
	})

	if err := e.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	e.Shutdown(1 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	if !fatalCalled {
		t.Error("expected onFatal to be called on timeout")
	}
}
