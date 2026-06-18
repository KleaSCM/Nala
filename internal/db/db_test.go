package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDBCreatesFile(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	dbPath := filepath.Join(dir, "nala.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestMigrateAppliesAll(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count); err != nil {
		t.Fatalf("cannot query _migrations: %v", err)
	}
	expected := len(Migrations())
	if count != expected {
		t.Errorf("expected %d migrations, got %d", expected, count)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate() error: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate() (idempotent) error: %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	expected := len(Migrations())
	if count != expected {
		t.Errorf("expected %d migrations after second run, got %d", expected, count)
	}
}

func TestMigrationsCreateTables(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	tables := []string{"agents", "sessions", "messages", "tool_bindings", "knowledge_bases", "documents", "provider_configs", "scheduled_tasks", "audit_log"}
	for _, name := range tables {
		if !tableExists(db, name) {
			t.Errorf("table %s does not exist after migration", name)
		}
	}
}

func tableExists(db *sql.DB, name string) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	return count > 0
}

func TestAgentCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	_, err = db.Exec(`INSERT INTO agents (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"test-id", "Test Agent", "test-agent")
	if err != nil {
		t.Fatalf("insert agent error: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM agents WHERE id = ?", "test-id").Scan(&name)
	if err != nil {
		t.Fatalf("query agent error: %v", err)
	}
	if name != "Test Agent" {
		t.Errorf("expected 'Test Agent', got %s", name)
	}
}

func TestDuplicateSlugErrors(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	_, err = db.Exec(`INSERT INTO agents (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"id-1", "Agent One", "same-slug")
	if err != nil {
		t.Fatalf("first insert error: %v", err)
	}

	_, err = db.Exec(`INSERT INTO agents (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"id-2", "Agent Two", "same-slug")
	if err == nil {
		t.Error("expected error on duplicate slug, got nil")
	}
}

func TestSessionCascadeDelete(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	db.Exec(`INSERT INTO agents (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"agent-1", "Test", "test")
	db.Exec(`INSERT INTO sessions (id, agent_id, created_at, updated_at) VALUES (?, ?, datetime('now'), datetime('now'))`,
		"session-1", "agent-1")

	db.Exec("DELETE FROM agents WHERE id = 'agent-1'")

	var count int
	db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'session-1'").Scan(&count)
	if count != 0 {
		t.Error("expected session to be cascade-deleted with agent")
	}
}
