package model

import (
	"context"
	"testing"
	"time"
)

func TestNewFallbackChain(t *testing.T) {
	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})
	if fc == nil {
		t.Fatal("expected fallback chain")
	}
	if fc.MaxRetries != 1 {
		t.Errorf("expected default MaxRetries 1, got %d", fc.MaxRetries)
	}
	if len(fc.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(fc.Steps))
	}
}

func TestFallbackChain_SetTimeout(t *testing.T) {
	fc := NewFallbackChain([]FallbackStep{{Provider: "ollama", Model: "qwen3.5:9b"}})
	fc.SetTimeout(5 * time.Second)
}

func TestIsRetryable_ProviderUnhealthy(t *testing.T) {
	if !isRetryable(ErrProviderUnhealthy) {
		t.Error("expected ErrProviderUnhealthy to be retryable")
	}
}

func TestIsRetryable_RateLimited(t *testing.T) {
	if !isRetryable(ErrRateLimited) {
		t.Error("expected ErrRateLimited to be retryable")
	}
}

func TestIsRetryable_ModelNotFound(t *testing.T) {
	if !isRetryable(ErrModelNotFound) {
		t.Error("expected ErrModelNotFound to be retryable")
	}
}

func TestIsRetryable_Refusal(t *testing.T) {
	if !isRetryable(ErrRefusalDetected) {
		t.Error("expected ErrRefusalDetected to be retryable")
	}
}

func TestIsRetryable_DeadlineExceeded(t *testing.T) {
	if !isRetryable(context.DeadlineExceeded) {
		t.Error("expected DeadlineExceeded to be retryable")
	}
}

func TestIsRetryable_Authentication(t *testing.T) {
	if isRetryable(ErrAuthentication) {
		t.Error("expected ErrAuthentication to NOT be retryable")
	}
}

func TestIsRetryable_Nil(t *testing.T) {
	if isRetryable(nil) {
		t.Error("expected nil to NOT be retryable")
	}
}

func TestIsRetryable_TimeoutMessage(t *testing.T) {
	err := NewMockProviderErr("timeout")
	if !isRetryable(err) {
		t.Error("expected timeout error to be retryable")
	}
}

func TestIsRetryable_ConnectionRefused(t *testing.T) {
	err := NewMockProviderErr("connection refused")
	if !isRetryable(err) {
		t.Error("expected connection refused to be retryable")
	}
}

func TestChatWithFallback_FirstStepSucceeds(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewMockProvider("ollama", "qwen3.5:9b", true))
	reg.Register(NewMockProvider("openai", "gpt-4o-mini", true))

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", result.Provider)
	}
	if result.Attempted != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempted)
	}
}

func TestChatWithFallback_FirstFailsThenSucceeds(t *testing.T) {
	reg := NewRegistry()
	unhealthy := NewMockProvider("ollama", "qwen3.5:9b", false)
	healthy := NewMockProvider("openai", "gpt-4o-mini", true)
	reg.Register(unhealthy)
	reg.Register(healthy)

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Provider != "openai" {
		t.Errorf("expected openai, got %s", result.Provider)
	}
	if result.Attempted != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempted)
	}
}

func TestChatWithFallback_AllFail(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewMockProvider("ollama", "qwen3.5:9b", false))
	reg.Register(NewMockProvider("openai", "gpt-4o-mini", false))

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Attempted != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempted)
	}
}

func TestChatWithFallback_NonRetryableStops(t *testing.T) {
	reg := NewRegistry()
	p := &configurableMockProvider{
		id:      "openai",
		name:    "openai",
		modelID: "gpt-4o-mini",
		chatErr: ErrAuthentication,
	}
	reg.Register(p)

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Attempted != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempted)
	}
}

func TestChatWithFallback_RetriesOnRetryable(t *testing.T) {
	reg := NewRegistry()
	retryCount := 0
	p := &configurableMockProvider{
		id:      "ollama",
		name:    "ollama",
		modelID: "qwen3.5:9b",
		chatFn: func(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
			retryCount++
			if retryCount < 2 {
				return nil, ErrProviderUnhealthy
			}
			return &ChatResponse{Model: "qwen3.5:9b", Provider: "ollama"}, nil
		},
	}
	reg.Register(p)

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
	})
	fc.MaxRetries = 2

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if retryCount < 2 {
		t.Errorf("expected at least 2 calls (initial + 1 retry), got %d", retryCount)
	}
}

func TestChatWithFallback_ProviderNotFound(t *testing.T) {
	reg := NewRegistry()
	fc := NewFallbackChain([]FallbackStep{
		{Provider: "nonexistent", Model: "model"},
	})

	result := reg.ChatWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if result.Error == nil {
		t.Fatal("expected error")
	}
}

func TestChatStreamWithFallback_FirstStepSucceeds(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewMockProvider("ollama", "qwen3.5:9b", true))
	reg.Register(NewMockProvider("openai", "gpt-4o-mini", true))

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	ch, err := reg.ChatStreamWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("expected channel")
	}
	for range ch {
	}
}

func TestChatStreamWithFallback_FallbackOnFailure(t *testing.T) {
	reg := NewRegistry()
	unhealthy := NewMockProvider("ollama", "qwen3.5:9b", false)
	healthy := NewMockProvider("openai", "gpt-4o-mini", true)
	reg.Register(unhealthy)
	reg.Register(healthy)

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	ch, err := reg.ChatStreamWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
}

func TestChatStreamWithFallback_AllExhausted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewMockProvider("ollama", "qwen3.5:9b", false))
	reg.Register(NewMockProvider("openai", "gpt-4o-mini", false))

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "ollama", Model: "qwen3.5:9b"},
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	_, err := reg.ChatStreamWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatStreamWithFallback_NonRetryableStops(t *testing.T) {
	reg := NewRegistry()
	p := &configurableMockProvider{
		id:        "openai",
		name:      "openai",
		modelID:   "gpt-4o-mini",
		streamErr: ErrAuthentication,
	}
	reg.Register(p)

	fc := NewFallbackChain([]FallbackStep{
		{Provider: "openai", Model: "gpt-4o-mini"},
	})

	_, err := reg.ChatStreamWithFallback(context.Background(), fc, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error for non-retryable")
	}
}
