package agent

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/KleaSCM/nala/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if _, err := d.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if _, err := d.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("foreign keys: %v", err)
	}
	for _, m := range db.Migrations() {
		if _, err := d.Exec(m.Query); err != nil {
			t.Fatalf("migration %d: %v", m.ID, err)
		}
	}
	return d
}

func TestManagerCreate(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Test Agent", Slug: "test-agent"}
	if err := m.Create(a); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if a.ID == "" {
		t.Error("expected ID to be set")
	}
	if a.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestManagerCreateDefaults(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Defaults Agent"}
	if err := m.Create(a); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if a.MaxTokens != 4096 {
		t.Errorf("expected default MaxTokens 4096, got %d", a.MaxTokens)
	}
	if a.Temperature != 0.7 {
		t.Errorf("expected default Temperature 0.7, got %f", a.Temperature)
	}
	if a.TimeoutMS != 300000 {
		t.Errorf("expected default TimeoutMS 300000, got %d", a.TimeoutMS)
	}
}

func TestManagerGet(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Get Test", Slug: "get-test"}
	m.Create(a)

	got, err := m.Get(a.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Name != "Get Test" {
		t.Errorf("expected 'Get Test', got %q", got.Name)
	}
}

func TestManagerGetNotFound(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestManagerGetBySlug(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Slug Test", Slug: "slug-test"}
	m.Create(a)

	got, err := m.GetBySlug("slug-test")
	if err != nil {
		t.Fatalf("GetBySlug error: %v", err)
	}
	if got.Name != "Slug Test" {
		t.Errorf("expected 'Slug Test', got %q", got.Name)
	}
}

func TestManagerUpdate(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Update Test", Slug: "update-test"}
	m.Create(a)

	a.Name = "Updated Name"
	if err := m.Update(a); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	got, _ := m.Get(a.ID)
	if got.Name != "Updated Name" {
		t.Errorf("expected 'Updated Name', got %q", got.Name)
	}
}

func TestManagerDelete(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	a := &Agent{Name: "Delete Test", Slug: "delete-test"}
	m.Create(a)

	if err := m.Delete(a.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	_, err := m.Get(a.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestManagerList(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	agents := []*Agent{
		{Name: "Alpha", Slug: "alpha"},
		{Name: "Beta", Slug: "beta"},
		{Name: "Gamma", Slug: "gamma"},
	}
	for _, a := range agents {
		m.Create(a)
	}

	list, err := m.List(AgentFilter{})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 agents, got %d", len(list))
	}
}

func TestManagerListWithFilter(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	m.Create(&Agent{Name: "Alpha Agent", Slug: "alpha"})
	m.Create(&Agent{Name: "Beta Agent", Slug: "beta"})
	m.Create(&Agent{Name: "Gamma", Slug: "gamma"})

	list, err := m.List(AgentFilter{Name: "Agent"})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 agents matching 'Agent', got %d", len(list))
	}
}

func TestManagerDuplicateSlug(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	m.Create(&Agent{Name: "Original", Slug: "same-slug"})
	a2 := &Agent{Name: "Duplicate", Slug: "same-slug"}
	if err := m.Create(a2); err != nil {
		t.Fatalf("Create duplicate slug error: %v", err)
	}
	if a2.Slug == "same-slug" {
		t.Errorf("expected slug to be modified, got %q", a2.Slug)
	}
	if !strings.HasPrefix(a2.Slug, "same-slug-") {
		t.Errorf("expected slug prefix 'same-slug-', got %q", a2.Slug)
	}
}

func TestManagerEmptyList(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)

	list, err := m.List(AgentFilter{})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Errorf("expected empty list, got %v", list)
	}
}

func TestSlugGeneration(t *testing.T) {
	d := setupTestDB(t)
	s, err := nextSlug(d, "my-agent")
	if err != nil {
		t.Fatalf("nextSlug error: %v", err)
	}
	if s != "my-agent" {
		t.Errorf("expected 'my-agent', got %q", s)
	}
}

func TestSessionManagerCreate(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Session Agent", Slug: "session-agent"}
	m.Create(a)

	s := &Session{AgentID: a.ID}
	if err := sm.Create(s); err != nil {
		t.Fatalf("Create session error: %v", err)
	}
	if s.ID == "" {
		t.Error("expected session ID to be set")
	}
	if s.Status != "active" {
		t.Errorf("expected status 'active', got %q", s.Status)
	}
}

func TestSessionManagerGet(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Get Session", Slug: "get-session"}
	m.Create(a)
	s := &Session{AgentID: a.ID, Title: "Test Session"}
	sm.Create(s)

	got, err := sm.Get(s.ID)
	if err != nil {
		t.Fatalf("Get session error: %v", err)
	}
	if got.Title != "Test Session" {
		t.Errorf("expected 'Test Session', got %q", got.Title)
	}
}

func TestSessionManagerPauseResume(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Pause Agent", Slug: "pause-agent"}
	m.Create(a)
	s := &Session{AgentID: a.ID}
	sm.Create(s)

	if err := sm.Pause(s.ID); err != nil {
		t.Fatalf("Pause error: %v", err)
	}
	got, _ := sm.Get(s.ID)
	if got.Status != "paused" {
		t.Errorf("expected 'paused', got %q", got.Status)
	}
	if got.PausedAt == nil {
		t.Error("expected PausedAt to be set")
	}

	if err := sm.Resume(s.ID); err != nil {
		t.Fatalf("Resume error: %v", err)
	}
	got, _ = sm.Get(s.ID)
	if got.Status != "active" {
		t.Errorf("expected 'active', got %q", got.Status)
	}
}

func TestSessionManagerComplete(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Complete Agent", Slug: "complete-agent"}
	m.Create(a)
	s := &Session{AgentID: a.ID}
	sm.Create(s)

	sm.Complete(s.ID)
	got, _ := sm.Get(s.ID)
	if got.Status != "completed" {
		t.Errorf("expected 'completed', got %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSessionManagerDelete(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Delete Session", Slug: "delete-session"}
	m.Create(a)
	s := &Session{AgentID: a.ID}
	sm.Create(s)

	if err := sm.Delete(s.ID); err != nil {
		t.Fatalf("Delete session error: %v", err)
	}
	_, err := sm.Get(s.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestSessionManagerList(t *testing.T) {
	d := setupTestDB(t)
	sm := NewSessionManager(d)
	m := NewManager(d)

	a := &Agent{Name: "List Agent", Slug: "list-agent"}
	m.Create(a)

	for i := 0; i < 3; i++ {
		sm.Create(&Session{AgentID: a.ID, Title: fmt.Sprintf("Session %d", i+1)})
	}

	list, err := sm.List(SessionFilter{AgentID: a.ID})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(list))
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter()
	if !rl.Allow("test", 2, time.Second) {
		t.Error("expected first request to be allowed")
	}
	if !rl.Allow("test", 2, time.Second) {
		t.Error("expected second request to be allowed")
	}
	if rl.Allow("test", 2, time.Second) {
		t.Error("expected third request to be denied")
	}
}

func TestRenderSystemPrompt(t *testing.T) {
	vars := DefaultTemplateVars()
	vars.AgentName = "Nala"
	vars.UserName = "Alice"
	result := RenderSystemPrompt("Hello {{user_name}}, I am {{agent_name}}.", vars)
	if result != "Hello Alice, I am Nala." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRenderSystemPromptCustomVar(t *testing.T) {
	vars := DefaultTemplateVars()
	vars.Custom["project"] = "Nala"
	result := RenderSystemPrompt("Working on {{project}}.", vars)
	if result != "Working on Nala." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestResolvePresetDefault(t *testing.T) {
	p := ResolvePreset("default")
	if p.Name != "default" {
		t.Errorf("expected 'default', got %q", p.Name)
	}
}

func TestResolvePresetCode(t *testing.T) {
	p := ResolvePreset("code")
	if p.Name != "code" {
		t.Errorf("expected 'code', got %q", p.Name)
	}
	if !strings.Contains(p.SystemPrompt, "software engineer") {
		t.Error("expected code preset to mention software engineer")
	}
}

func TestResolvePresetUnknown(t *testing.T) {
	p := ResolvePreset("nonexistent")
	if p.Name != "default" {
		t.Errorf("expected fallback to 'default', got %q", p.Name)
	}
}

func TestSessionManagerSetTitle(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Title Agent", Slug: "title-agent"}
	m.Create(a)
	s := &Session{AgentID: a.ID}
	sm.Create(s)

	sm.SetTitle(s.ID, "My Chat Session")
	got, _ := sm.Get(s.ID)
	if got.Title != "My Chat Session" {
		t.Errorf("expected 'My Chat Session', got %q", got.Title)
	}
}

func TestSessionManagerTrackUsage(t *testing.T) {
	d := setupTestDB(t)
	m := NewManager(d)
	sm := NewSessionManager(d)

	a := &Agent{Name: "Usage Agent", Slug: "usage-agent"}
	m.Create(a)
	s := &Session{AgentID: a.ID}
	sm.Create(s)

	sm.TrackUsage(s.ID, 100, 50, 0.001)
	got, _ := sm.Get(s.ID)
	if got.MessageCount != 1 {
		t.Errorf("expected message count 1, got %d", got.MessageCount)
	}
	if got.TotalTokensIn != 100 {
		t.Errorf("expected 100 tokens in, got %d", got.TotalTokensIn)
	}
	if got.TotalTokensOut != 50 {
		t.Errorf("expected 50 tokens out, got %d", got.TotalTokensOut)
	}
}
