package tool

import (
	"context"
	"encoding/json"
	"testing"
)

type mockTool struct{}

func (m *mockTool) ID() string                       { return "web.search" }
func (m *mockTool) Name() string                     { return "Web Search" }
func (m *mockTool) Description() string               { return "Search the web" }
func (m *mockTool) ParameterSchema() json.RawMessage  { return json.RawMessage(`{"type":"object"}`) }
func (m *mockTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	return &Result{Success: true, Content: "search results"}, nil
}
func (m *mockTool) ValidateArgs(args json.RawMessage) error { return nil }

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	m := &mockTool{}
	if err := r.Register(m); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	got, err := r.Get("web.search")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID() != "web.search" {
		t.Errorf("expected web.search, got %s", got.ID())
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	m := &mockTool{}
	r.Register(m)
	if err := r.Register(m); err == nil {
		t.Error("expected error on duplicate register")
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	list := r.List("")
	if len(list) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(list))
	}
	if list[0].ID != "web.search" {
		t.Errorf("expected web.search, got %s", list[0].ID)
	}
	if list[0].Category != "web" {
		t.Errorf("expected category 'web', got %s", list[0].Category)
	}
}

func TestListByCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	list := r.List("web")
	if len(list) != 1 {
		t.Fatalf("expected 1 tool in web category, got %d", len(list))
	}
	list = r.List("file")
	if len(list) != 0 {
		t.Errorf("expected 0 tools in file category, got %d", len(list))
	}
}

func TestCategories(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	cats := r.Categories()
	if len(cats) != 1 || cats[0] != "web" {
		t.Errorf("expected [web], got %v", cats)
	}
}

func TestUnregister(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	r.Unregister("web.search")
	_, err := r.Get("web.search")
	if err == nil {
		t.Error("expected error after unregister")
	}
}

func TestToolCategory(t *testing.T) {
	if c := toolCategory("web.search"); c != "web" {
		t.Errorf("expected 'web', got %q", c)
	}
	if c := toolCategory("file.read"); c != "file" {
		t.Errorf("expected 'file', got %q", c)
	}
	if c := toolCategory("no-dot"); c != "uncategorized" {
		t.Errorf("expected 'uncategorized', got %q", c)
	}
}

func TestExecute(t *testing.T) {
	m := &mockTool{}
	result, err := m.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Content != "search results" {
		t.Errorf("expected 'search results', got %q", result.Content)
	}
}
