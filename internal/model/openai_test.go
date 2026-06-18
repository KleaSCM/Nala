package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	p := NewOpenAIProvider("", "sk-test")
	if p.endpoint != DefaultOpenAIEndpoint {
		t.Errorf("expected default endpoint, got %q", p.endpoint)
	}

	p2 := NewOpenAIProvider("https://custom.example.com/v1", "sk-test")
	if p2.endpoint != "https://custom.example.com/v1" {
		t.Errorf("expected custom endpoint, got %q", p2.endpoint)
	}
}

func TestOpenAIValidateConfig(t *testing.T) {
	p := NewOpenAIProvider("", "")
	if err := p.ValidateConfig(); err == nil {
		t.Error("expected error for empty api key")
	}

	p2 := NewOpenAIProvider("", "sk-valid")
	if err := p2.ValidateConfig(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenAIListModels(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer token")
		}
		json.NewEncoder(w).Encode(openAIModelsResp{
			Data: []openAIModelEntry{
				{ID: "gpt-4o"},
				{ID: "gpt-4o-mini"},
				{ID: "o1"},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels error: %v", err)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call, got %d", callCount.Load())
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].ID != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", models[0].ID)
	}
	if models[0].Provider != "openai" {
		t.Errorf("expected openai provider, got %s", models[0].Provider)
	}
	if models[0].CostInput == 0 {
		t.Error("expected non-zero CostInput for gpt-4o")
	}
	if models[2].ContextLen != 200000 {
		t.Errorf("expected o1 context len 200000, got %d", models[2].ContextLen)
	}
}

func TestOpenAIListModelsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openAIError{Message: "invalid API key"})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-bad")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenAIChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer token")
		}
		var req openAIChatReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Stream {
			t.Error("expected non-streaming request")
		}
		if req.Model != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %s", req.Model)
		}
		content := "Hello! How can I help you today?"
		json.NewEncoder(w).Encode(openAIChatResp{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIRespMsg{
						Role:    "assistant",
						Content: &content,
					},
					FinishReason: "stop",
				},
			},
			Usage: &openAIUsage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Message.Content != "Hello! How can I help you today?" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", resp.Model)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("expected 18 total tokens, got %d", resp.Usage.TotalTokens)
	}
	if resp.Duration < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestOpenAIChatWithToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatReq
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		args := `{"location":"Tokyo"}`
		json.NewEncoder(w).Encode(openAIChatResp{
			ID:    "chatcmpl-tool",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIRespMsg{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: openAIFunctionCall{
									Name:      "get_weather",
									Arguments: args,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
		Tools: []ToolDef{
			{
				Function: ToolFunctionDef{
					Name:        "get_weather",
					Description: "Get weather",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].ID != "call_123" {
		t.Errorf("expected call_123, got %s", resp.Message.ToolCalls[0].ID)
	}
	if resp.Message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", resp.Message.ToolCalls[0].Function.Name)
	}
}

func TestOpenAIChatRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(openAIError{
			Message: "Rate limit exceeded",
			Type:    "rate_limit",
			Code:    "rate_limit_exceeded",
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestOpenAIChatAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openAIError{
			Message: "Invalid API key",
			Type:    "authentication_error",
			Code:    "invalid_api_key",
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-bad")
	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestOpenAIChatEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openAIChatResp{
			ID:      "chatcmpl-empty",
			Model:   "gpt-4o",
			Choices: []openAIChoice{},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestOpenAIChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer token")
		}
		var req openAIChatReq
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected streaming request")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []string{
			`data: {"id":"chatcmpl-1","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-1","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-1","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			"data: [DONE]",
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	ch, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}

	var tokens []string
	var usage *TokenUsage
	gotDone := false
	for delta := range ch {
		if delta.Done {
			if delta.Usage != nil {
				usage = delta.Usage
			}
			gotDone = true
			continue
		}
		if delta.Type == "token" && delta.Content != "" {
			tokens = append(tokens, delta.Content)
		}
		if delta.Error != "" {
			t.Errorf("unexpected error: %s", delta.Error)
		}
	}
	if !gotDone {
		t.Error("expected done signal")
	}
	got := strings.Join(tokens, "")
	if got != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", got)
	}
	if usage == nil || usage.TotalTokens != 7 {
		t.Errorf("expected usage total 7, got %+v", usage)
	}
}

func TestOpenAIChatStreamToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []string{
			`data: {"id":"chatcmpl-tc","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-tc","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"loc"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-tc","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"arguments":"ation\":\"Tokyo\"}"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-tc","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			"data: [DONE]",
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	ch, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Weather in Tokyo"}},
		Tools: []ToolDef{
			{
				Function: ToolFunctionDef{
					Name:        "get_weather",
					Description: "Get weather for a location",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}

	toolCallStarted := false
	done := false
	for delta := range ch {
		if delta.Type == "tool_call_start" {
			toolCallStarted = true
			if len(delta.ToolCalls) == 0 {
				t.Error("expected tool calls in delta")
			}
		}
		if delta.Done {
			done = true
		}
	}
	if !toolCallStarted {
		t.Error("expected tool_call_start delta")
	}
	if !done {
		t.Error("expected done signal")
	}
}

func TestOpenAIChatStreamRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	_, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestOpenAIChatStreamAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-bad")
	_, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestOpenAIChatStreamConnectionError(t *testing.T) {
	p := NewOpenAIProvider("http://localhost:1", "sk-test")
	_, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestOpenAICountTokens(t *testing.T) {
	p := NewOpenAIProvider("", "sk-test")
	msgs := []Message{
		{Role: "user", Content: "Hello world"},
	}
	n, err := p.CountTokens(context.Background(), msgs)
	if err != nil {
		t.Fatalf("CountTokens error: %v", err)
	}
	if n <= 0 {
		t.Errorf("expected positive token count, got %d", n)
	}
}

func TestOpenAICountTokensWithToolCalls(t *testing.T) {
	p := NewOpenAIProvider("", "sk-test")
	msgs := []Message{
		{Role: "user", Content: "What's the weather?"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "get_weather",
						Arguments: `{"location":"Tokyo"}`,
					},
				},
			},
		},
	}
	n, err := p.CountTokens(context.Background(), msgs)
	if err != nil {
		t.Fatalf("CountTokens error: %v", err)
	}
	if n <= 0 {
		t.Errorf("expected positive token count, got %d", n)
	}
}

func TestOpenAICostForTokens(t *testing.T) {
	p := NewOpenAIProvider("", "sk-test")
	cost, err := p.CostForTokens("gpt-4o", 1000, 500)
	if err != nil {
		t.Fatalf("CostForTokens error: %v", err)
	}
	expected := (2.50*1000/1000 + 10.00*500/1000)
	if cost != expected {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}

	_, err = p.CostForTokens("unknown-model", 100, 100)
	if err == nil {
		t.Error("expected error for unknown model cost lookup")
	}
}

func TestOpenAIChatWithSystemPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatReq
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages (system + user), got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected first message role 'system', got %q", req.Messages[0].Role)
		}
		var sysContent string
		json.Unmarshal(req.Messages[0].Content, &sysContent)
		if sysContent != "Be helpful" {
			t.Errorf("expected system prompt 'Be helpful', got %q", sysContent)
		}
		content := "Sure, I'll help!"
		json.NewEncoder(w).Encode(openAIChatResp{
			ID:    "chatcmpl-sys",
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIRespMsg{
						Role:    "assistant",
						Content: &content,
					},
				},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "sk-test")
	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:        "gpt-4o",
		SystemPrompt: "Be helpful",
		Messages:     []Message{{Role: "user", Content: "Help me"}},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Message.Content != "Sure, I'll help!" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
}

func TestOpenAIImplementsProvider(t *testing.T) {
	var p Provider = NewOpenAIProvider("", "sk-test")
	_ = p
}
