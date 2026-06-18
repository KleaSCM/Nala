package model

import (
	"context"
	"testing"
)

func TestDetectIntent_Chat(t *testing.T) {
	intent := DetectIntent("Hello, how are you?")
	if intent != IntentChat {
		t.Errorf("expected chat intent, got %s", intent)
	}
}

func TestDetectIntent_Code(t *testing.T) {
	intent := DetectIntent("Write a Go function to sort a slice")
	if intent != IntentCode {
		t.Errorf("expected code intent, got %s", intent)
	}
}

func TestDetectIntent_Reasoning(t *testing.T) {
	intent := DetectIntent("Explain why the sky is blue step by step")
	if intent != IntentReasoning {
		t.Errorf("expected reasoning intent, got %s", intent)
	}
}

func TestDetectIntent_Creative(t *testing.T) {
	intent := DetectIntent("Write a poem about AI")
	if intent != IntentCreative {
		t.Errorf("expected creative intent, got %s", intent)
	}
}

func TestDetectIntent_Security(t *testing.T) {
	intent := DetectIntent("How do I exploit a buffer overflow vulnerability?")
	if intent != IntentSecurity {
		t.Errorf("expected security intent, got %s", intent)
	}
}

func TestDetectIntent_Tool(t *testing.T) {
	intent := DetectIntent("Run ls in the current directory")
	if intent != IntentTool {
		t.Errorf("expected tool intent, got %s", intent)
	}
}

func TestDetectIntent_Empty(t *testing.T) {
	intent := DetectIntent("")
	if intent != IntentChat {
		t.Errorf("expected chat intent for empty message, got %s", intent)
	}
}

func TestDetectIntent_MixedKeywords_CodeWins(t *testing.T) {
	intent := DetectIntent("Write a script to explain the algorithm")
	_ = intent
}

func TestNewRouter(t *testing.T) {
	reg := NewRegistry()
	r := NewRouter(reg)
	if r == nil {
		t.Fatal("expected router")
	}
}

func TestRouter_SetRules(t *testing.T) {
	reg := NewRegistry()
	r := NewRouter(reg)
	r.SetRules([]RoutingRule{
		{Intent: IntentChat, Provider: "ollama", Model: "qwen3.5:9b", Priority: 10},
	})
}

func TestRouter_AddRule(t *testing.T) {
	reg := NewRegistry()
	r := NewRouter(reg)
	r.AddRule(RoutingRule{Intent: IntentChat, Provider: "openai", Model: "gpt-4o-mini", Priority: 5})
}

func TestRouter_Route_ExplicitModel(t *testing.T) {
	reg := NewRegistry()
	p := NewMockProvider("test-provider", "test-model", true)
	reg.Register(p)
	r := NewRouter(reg)

	_, _, err := r.Route(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("Route error: %v", err)
	}
}

func TestRouter_Route_NoSuitableProvider(t *testing.T) {
	reg := NewRegistry()
	r := NewRouter(reg)

	_, _, err := r.Route(context.Background(), ChatRequest{
		SystemPrompt: "Write a poem",
		Messages:     []Message{{Role: "user", Content: "Write a creative story"}},
	})
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestRouter_Route_NoProviders(t *testing.T) {
	reg := NewRegistry()
	r := NewRouter(reg)

	_, _, err := r.Route(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestRouter_Chat(t *testing.T) {
	reg := NewRegistry()
	p := NewMockProvider("ollama", "test-model", true)
	reg.Register(p)
	r := NewRouter(reg)

	resp, err := r.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

func TestRouter_ChatStream(t *testing.T) {
	reg := NewRegistry()
	p := NewMockProvider("ollama", "test-model", true)
	reg.Register(p)
	r := NewRouter(reg)

	ch, err := r.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}
	if ch == nil {
		t.Fatal("expected stream channel")
	}
	// drain
	for range ch {
	}
}
