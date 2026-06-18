/**
 * Tests for the Ollama provider with a mock HTTP server.
 * モックHTTPサーバーを使ったOllamaプロバイダのテストね。
 *
 * Covers Chat, ChatStream, ListModels, error cases (connection refused,
 * model not found), and edge cases (empty response, streaming parse).
 * Chat、ChatStream、ListModels、エラーケース（接続拒否、モデル未発見）、
 * エッジケース（空レスポンス、ストリーム解析）をカバーしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newOllamaTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *OllamaProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return NewOllamaProvider(srv.URL)
}

func TestOllamaListModels(t *testing.T) {
	p := newOllamaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("expected /api/tags, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ollamaTagsResp{
			Models: []ollamaModel{
				{Name: "llama3.2:3b", Size: 2000000000},
				{Name: "mistral:7b", Size: 4000000000},
			},
		})
	})

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "llama3.2:3b" {
		t.Errorf("expected llama3.2:3b, got %s", models[0].ID)
	}
}

func TestOllamaChat(t *testing.T) {
	p := newOllamaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ollamaChatResp{
			Model: "llama3.2:3b",
			Message: ollamaMsg{
				Role:    "assistant",
				Content: "Hello! How can I help you?",
			},
			Done:             true,
			EvalCount:        10,
			PromptEvalCount:  5,
		})
	})

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model: "llama3.2:3b",
		Messages: []Message{
			{Role: "user", Content: "Say hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 5 {
		t.Errorf("expected 5 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 10 {
		t.Errorf("expected 10 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestOllamaChatWithSystemPrompt(t *testing.T) {
	p := newOllamaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatReq
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages (system+user), got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected first message to be system, got %s", req.Messages[0].Role)
		}

		json.NewEncoder(w).Encode(ollamaChatResp{
			Model: "llama3.2:3b",
			Message: ollamaMsg{Role: "assistant", Content: "Got it"},
			Done:  true,
		})
	})

	resp, err := p.Chat(context.Background(), ChatRequest{
		Model:        "llama3.2:3b",
		SystemPrompt: "You are a helpful assistant.",
		Messages:     []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Message.Content != "Got it" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
}

func TestOllamaChatModelNotFound(t *testing.T) {
	p := newOllamaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := p.Chat(context.Background(), ChatRequest{Model: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for model not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestOllamaChatStream(t *testing.T) {
	p := newOllamaTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatReq
		json.NewDecoder(r.Body).Decode(&req)

		if !req.Stream {
			t.Error("expected stream=true")
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		enc.Encode(ollamaChatResp{
			Model:   "llama3.2:3b",
			Message: ollamaMsg{Role: "assistant", Content: "Hel"},
			Done:    false,
		})
		enc.Encode(ollamaChatResp{
			Model:   "llama3.2:3b",
			Message: ollamaMsg{Role: "assistant", Content: "lo!"},
			Done:    false,
		})
		enc.Encode(ollamaChatResp{
			Model:   "llama3.2:3b",
			Message: ollamaMsg{Role: "assistant", Content: ""},
			Done:    true,
			EvalCount: 5,
			PromptEvalCount: 3,
		})
	})

	ch, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "llama3.2:3b",
		Messages: []Message{{Role: "user", Content: "Say hello"}},
	})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}

	var content strings.Builder
	var usage *TokenUsage
	for delta := range ch {
		content.WriteString(delta.Content)
		if delta.Usage != nil {
			usage = delta.Usage
		}
		if delta.Error != "" {
			t.Fatalf("stream error: %s", delta.Error)
		}
	}

	if content.String() != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", content.String())
	}
	if usage == nil {
		t.Fatal("expected usage info")
	}
	if usage.TotalTokens != 8 {
		t.Errorf("expected 8 total tokens, got %d", usage.TotalTokens)
	}
}

func TestOllamaConnectionRefused(t *testing.T) {
	p := NewOllamaProvider("http://localhost:1")

	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "cannot connect") {
		t.Errorf("expected 'cannot connect' in error, got: %v", err)
	}
}

func TestOllamaValidateConfig(t *testing.T) {
	p := NewOllamaProvider("")
	if err := p.ValidateConfig(); err != nil {
		t.Fatalf("ValidateConfig error: %v", err)
	}

	empty := NewOllamaProvider("")
	if err := empty.ValidateConfig(); err != nil {
		t.Fatalf("empty endpoint should be valid (defaults to localhost): %v", err)
	}
}

func TestOllamaCountTokens(t *testing.T) {
	p := NewOllamaProvider("")
	count, err := p.CountTokens(context.Background(), []Message{
		{Role: "user", Content: "hello world"},
	})
	if err != nil {
		t.Fatalf("CountTokens error: %v", err)
	}
	if count == 0 {
		t.Error("expected non-zero token count")
	}
}

func TestOllamaDefaultEndpoint(t *testing.T) {
	p := NewOllamaProvider("")
	if p.endpoint != DefaultOllamaEndpoint {
		t.Errorf("expected %s, got %s", DefaultOllamaEndpoint, p.endpoint)
	}
}

func TestOllamaProviderID(t *testing.T) {
	p := NewOllamaProvider("")
	if p.ID() != "ollama" {
		t.Errorf("expected ollama, got %s", p.ID())
	}
}
