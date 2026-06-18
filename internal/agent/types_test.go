package agent

import "testing"

func TestAgentValidDefaults(t *testing.T) {
	a := Agent{
		Name:        "Test Agent",
		Slug:        "test-agent",
		Temperature: 0.7,
		TopP:        0.9,
		TimeoutMS:   300000,
		MaxRetries:  3,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestAgentEmptyName(t *testing.T) {
	a := Agent{Slug: "test", Temperature: 0.7, TopP: 0.9, TimeoutMS: 300000, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestAgentNameTooLong(t *testing.T) {
	name := make([]byte, 101)
	for i := range name {
		name[i] = 'a'
	}
	a := Agent{Name: string(name), Slug: "test", Temperature: 0.7, TopP: 0.9, TimeoutMS: 300000, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for name > 100 chars")
	}
}

func TestAgentInvalidSlug(t *testing.T) {
	a := Agent{Name: "Test", Slug: "UPPERCASE", Temperature: 0.7, TopP: 0.9, TimeoutMS: 300000, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for uppercase slug")
	}
}

func TestAgentTemperatureOutOfRange(t *testing.T) {
	a := Agent{Name: "Test", Slug: "test", Temperature: 3.0, TopP: 0.9, TimeoutMS: 300000, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for temperature > 2.0")
	}
}

func TestAgentTopPOutOfRange(t *testing.T) {
	a := Agent{Name: "Test", Slug: "test", Temperature: 0.7, TopP: 1.5, TimeoutMS: 300000, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for top_p > 1.0")
	}
}

func TestAgentTimeoutOutOfRange(t *testing.T) {
	a := Agent{Name: "Test", Slug: "test", Temperature: 0.7, TopP: 0.9, TimeoutMS: 500, MaxRetries: 3}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for timeout < 1000")
	}
}

func TestAgentMaxRetriesOutOfRange(t *testing.T) {
	a := Agent{Name: "Test", Slug: "test", Temperature: 0.7, TopP: 0.9, TimeoutMS: 300000, MaxRetries: 15}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for max_retries > 10")
	}
}

func TestAgentEdgeCaseEmptySystemPrompt(t *testing.T) {
	a := Agent{
		Name:         "Test",
		Slug:         "test",
		SystemPrompt: "",
		Temperature:  0.7,
		TopP:         0.9,
		TimeoutMS:    300000,
		MaxRetries:   3,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("empty system prompt should be valid: %v", err)
	}
}

func TestAgentEdgeCaseEmptyToolBindings(t *testing.T) {
	a := Agent{
		Name:         "Test",
		Slug:         "test",
		ToolBindings: "[]",
		Temperature:  0.7,
		TopP:         0.9,
		TimeoutMS:    300000,
		MaxRetries:   3,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("empty tool bindings should be valid: %v", err)
	}
}

func TestSessionValidStatuses(t *testing.T) {
	valid := []string{"active", "paused", "completed", "archived"}
	for _, s := range valid {
		sess := Session{Status: s, MaxContextSize: 32000}
		if err := sess.Validate(); err != nil {
			t.Errorf("expected valid status %q: %v", s, err)
		}
	}
}

func TestSessionInvalidStatus(t *testing.T) {
	sess := Session{Status: "invalid", MaxContextSize: 32000}
	if err := sess.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestSessionContextSizeOutOfRange(t *testing.T) {
	sess := Session{Status: "active", MaxContextSize: 100}
	if err := sess.Validate(); err == nil {
		t.Fatal("expected error for max_context_size < 1024")
	}
}

func TestMessageValidRoles(t *testing.T) {
	valid := []string{"user", "assistant", "system", "tool"}
	for _, r := range valid {
		msg := Message{Role: r}
		if err := msg.Validate(); err != nil {
			t.Errorf("expected valid role %q: %v", r, err)
		}
	}
}

func TestMessageInvalidRole(t *testing.T) {
	msg := Message{Role: "bot"}
	if err := msg.Validate(); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestToolBindingEmptyToolID(t *testing.T) {
	tb := ToolBinding{ToolID: ""}
	if err := tb.Validate(); err == nil {
		t.Fatal("expected error for empty tool_id")
	}
}

func TestToolBindingValid(t *testing.T) {
	tb := ToolBinding{ToolID: "shell.run", Enabled: true}
	if err := tb.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
