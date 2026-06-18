package model

import (
	"testing"
)

func TestNewTokenTracker(t *testing.T) {
	tt := NewTokenTracker()
	if tt == nil {
		t.Fatal("expected token tracker")
	}
	if tt.GlobalTokens == nil {
		t.Fatal("expected global tokens to be initialized")
	}
}

func TestTokenTracker_TrackMessage(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("session-1", "openai", "gpt-4o", TokenUsage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	})

	su := tt.SessionUsage("session-1")
	if su.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", su.InputTokens)
	}
	if su.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", su.OutputTokens)
	}
	if su.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", su.TotalTokens)
	}
	if su.MessageCount != 1 {
		t.Errorf("expected 1 message, got %d", su.MessageCount)
	}
}

func TestTokenTracker_GlobalTracking(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("s1", "openai", "gpt-4o-mini", TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	})
	tt.TrackMessage("s2", "ollama", "qwen3.5:9b", TokenUsage{
		InputTokens:  200,
		OutputTokens: 100,
		TotalTokens:  300,
	})

	gu := tt.GlobalUsage()
	if gu.InputTokens != 300 {
		t.Errorf("expected 300 global input, got %d", gu.InputTokens)
	}
	if gu.OutputTokens != 150 {
		t.Errorf("expected 150 global output, got %d", gu.OutputTokens)
	}
	if gu.TotalTokens != 450 {
		t.Errorf("expected 450 global total, got %d", gu.TotalTokens)
	}
	if gu.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", gu.MessageCount)
	}
}

func TestTokenTracker_CostTracking(t *testing.T) {
	tt := NewTokenTracker()
	// gpt-4o-mini: $0.15/1K input, $0.60/1K output
	tt.TrackMessage("s1", "openai", "gpt-4o-mini", TokenUsage{
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	})

	su := tt.SessionUsage("s1")
	if su.TotalCost <= 0 {
		t.Errorf("expected positive cost, got %f", su.TotalCost)
	}
	// expected: (0.15 * 1000/1000) + (0.60 * 500/1000) = 0.15 + 0.30 = 0.45
	expected := 0.15 + 0.30
	if su.TotalCost != expected {
		t.Errorf("expected cost %f, got %f", expected, su.TotalCost)
	}

	gu := tt.GlobalUsage()
	if gu.TotalCost != expected {
		t.Errorf("expected global cost %f, got %f", expected, gu.TotalCost)
	}
}

func TestTokenTracker_SessionIsolation(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("s1", "openai", "gpt-4o", TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	tt.TrackMessage("s2", "ollama", "qwen3.5:9b", TokenUsage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30})

	s1 := tt.SessionUsage("s1")
	if s1.InputTokens != 10 {
		t.Errorf("expected s1 input 10, got %d", s1.InputTokens)
	}
	s2 := tt.SessionUsage("s2")
	if s2.InputTokens != 20 {
		t.Errorf("expected s2 input 20, got %d", s2.InputTokens)
	}
}

func TestTokenTracker_ResetSession(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("s1", "openai", "gpt-4o", TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	tt.TrackMessage("s1", "openai", "gpt-4o", TokenUsage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8})
	tt.TrackMessage("s2", "ollama", "qwen3.5:9b", TokenUsage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30})

	tt.ResetSession("s1")

	gu := tt.GlobalUsage()
	// global should only have s2's tokens
	if gu.InputTokens != 20 {
		t.Errorf("expected global input 20, got %d", gu.InputTokens)
	}
	if gu.TotalTokens != 30 {
		t.Errorf("expected global total 30, got %d", gu.TotalTokens)
	}
	if gu.MessageCount != 1 {
		t.Errorf("expected message count 1, got %d", gu.MessageCount)
	}

	s1 := tt.SessionUsage("s1")
	if s1.TotalTokens != 0 {
		t.Errorf("expected s1 reset to 0, got %d", s1.TotalTokens)
	}
}

func TestTokenTracker_ResetAll(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("s1", "openai", "gpt-4o", TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	tt.TrackMessage("s2", "ollama", "qwen3.5:9b", TokenUsage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30})

	tt.ResetAll()

	gu := tt.GlobalUsage()
	if gu.TotalTokens != 0 {
		t.Errorf("expected all tokens reset, got %d", gu.TotalTokens)
	}
	if gu.MessageCount != 0 {
		t.Errorf("expected message count 0, got %d", gu.MessageCount)
	}
}

func TestTokenTracker_OllamaZeroCost(t *testing.T) {
	tt := NewTokenTracker()
	tt.TrackMessage("s1", "ollama", "qwen3.5:9b", TokenUsage{
		InputTokens:  10000,
		OutputTokens: 5000,
		TotalTokens:  15000,
	})

	su := tt.SessionUsage("s1")
	if su.TotalCost != 0 {
		t.Errorf("expected zero cost for ollama, got %f", su.TotalCost)
	}
}

func TestNewCostTable(t *testing.T) {
	ct := NewCostTable()
	if ct == nil {
		t.Fatal("expected cost table")
	}
}

func TestCostTable_SetAndGet(t *testing.T) {
	ct := NewCostTable()
	ct.SetCost("test", "my-model", 1.0/1000, 2.0/1000)
	cost := ct.CostFor("test", "my-model", 1000, 500)
	expected := (1.0/1000)*1000 + (2.0/1000)*500
	if cost != expected {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCostTable_UnknownProvider(t *testing.T) {
	ct := NewCostTable()
	cost := ct.CostFor("unknown", "some-model", 100, 100)
	if cost != 0 {
		t.Errorf("expected 0 for unknown provider, got %f", cost)
	}
}

func TestCostTable_WildcardFallback(t *testing.T) {
	ct := NewCostTable()
	cost := ct.CostFor("openai", "gpt-4o-unknown-version", 1000, 500)
	if cost <= 0 {
		t.Errorf("expected positive cost from wildcard, got %f", cost)
	}
}

func TestSessionTokenUsageEmpty(t *testing.T) {
	tt := NewTokenTracker()
	su := tt.SessionUsage("nonexistent")
	if su == nil {
		t.Fatal("expected empty usage, not nil")
	}
	if su.TotalTokens != 0 {
		t.Errorf("expected 0, got %d", su.TotalTokens)
	}
}
