package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestLoggerWritesJSON(t *testing.T) {
	out := captureStdout(func() {
		log, err := New("debug", "", 50, 30)
		if err != nil {
			t.Fatal(err)
		}
		log.Info("test message", "key", "value")
		log.Sync()
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one log line")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if entry["msg"] != "test message" {
		t.Errorf("expected msg='test message', got %v", entry["msg"])
	}
	if entry["level"] != "info" {
		t.Errorf("expected level=info, got %v", entry["level"])
	}
}

func TestLoggerWithAttachesFields(t *testing.T) {
	out := captureStdout(func() {
		log, err := New("debug", "", 50, 30)
		if err != nil {
			t.Fatal(err)
		}
		child := log.With("component", "test")
		child.Info("child message")
		child.Sync()
	})

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if entry["component"] != "test" {
		t.Errorf("expected component=test, got %v", entry["component"])
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	out := captureStdout(func() {
		log, err := New("warn", "", 50, 30)
		if err != nil {
			t.Fatal(err)
		}
		log.Debug("should not appear")
		log.Info("should not appear")
		log.Warn("should appear")
		log.Sync()
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 log line (warn only), got %d", len(lines))
	}
}

func TestLoggerFileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	log, err := New("info", logPath, 50, 30)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("file test")
	log.Sync()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
	if !strings.Contains(string(data), "file test") {
		t.Errorf("expected log file to contain 'file test', got %s", string(data))
	}
}
