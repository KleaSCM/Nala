/**
 * Tests for provider interface and core types.
 * プロバイダインターフェースとコアタイプのテストね。
 *
 * Covers Message construction, error sentinel values,
 * and parameter validation.
 * メッセージ構築、エラーセンチネル値、パラメータバリデーションをカバーしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type mockProvider struct{}

func (m *mockProvider) ID() string                                { return "mock" }
func (m *mockProvider) Name() string                              { return "Mock Provider" }
func (m *mockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) { return nil, nil }
func (m *mockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Model: "mock", Provider: "mock"}, nil
}
func (m *mockProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	ch := make(chan StreamDelta, 1)
	ch <- StreamDelta{Done: true}
	close(ch)
	return ch, nil
}
func (m *mockProvider) CountTokens(ctx context.Context, msgs []Message) (int, error) { return 0, nil }
func (m *mockProvider) ValidateConfig() error { return nil }

func TestMockProviderImplementsInterface(t *testing.T) {
	var p Provider = &mockProvider{}
	if p.ID() != "mock" {
		t.Errorf("expected mock, got %s", p.ID())
	}
}

func TestMessageConstruction(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "hello",
	}
	if msg.Role != "user" {
		t.Errorf("expected user, got %s", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("expected hello, got %s", msg.Content)
	}
}

func TestChatResponseHasFields(t *testing.T) {
	resp := &ChatResponse{
		Message:  Message{Role: "assistant", Content: "hi"},
		Usage:    TokenUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
		Model:    "test-model",
		Provider: "test-provider",
	}
	if resp.Message.Content != "hi" {
		t.Errorf("expected hi, got %s", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("expected 30, got %d", resp.Usage.TotalTokens)
	}
}

func TestStreamDeltaDone(t *testing.T) {
	delta := StreamDelta{Done: true, Usage: &TokenUsage{TotalTokens: 100}}
	if !delta.Done {
		t.Error("expected done to be true")
	}
	if delta.Usage.TotalTokens != 100 {
		t.Errorf("expected 100, got %d", delta.Usage.TotalTokens)
	}
}

func TestParametersDefaults(t *testing.T) {
	p := Parameters{
		Temperature: 0.7,
		MaxTokens:   4096,
	}
	if p.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", p.Temperature)
	}
}

func TestErrorSentinels(t *testing.T) {
	if !errors.Is(ErrProviderNotFound, ErrProviderNotFound) {
		t.Error("sentinel should match itself")
	}
	if !errors.Is(ErrRateLimited, ErrRateLimited) {
		t.Error("rate limit sentinel broken")
	}
	if !errors.Is(ErrRefusalDetected, ErrRefusalDetected) {
		t.Error("refusal sentinel broken")
	}
}

func TestModelInfoFields(t *testing.T) {
	mi := ModelInfo{
		ID:         "llama3.2:3b",
		Name:       "Llama 3.2 3B",
		Provider:   "ollama",
		ContextLen: 128000,
		Tags:       []string{"safe"},
	}
	if mi.ContextLen != 128000 {
		t.Errorf("expected 128000, got %d", mi.ContextLen)
	}
	if len(mi.Tags) != 1 || mi.Tags[0] != "safe" {
		t.Errorf("expected [safe], got %v", mi.Tags)
	}
}

func TestMockChatStream(t *testing.T) {
	p := &mockProvider{}
	ch, err := p.ChatStream(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}
	delta, ok := <-ch
	if !ok {
		t.Fatal("expected stream delta")
	}
	if !delta.Done {
		t.Error("expected done")
	}
}

// configurableMockProvider supports test-driven provider ID/model/error control.
type configurableMockProvider struct {
	id        string
	name      string
	modelID   string
	chatErr   error
	streamErr error
	chatFn    func(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	models    []ModelInfo
}

func NewMockProvider(id, modelID string, healthy bool) *configurableMockProvider {
	p := &configurableMockProvider{
		id:      id,
		name:    id,
		modelID: modelID,
		models:  []ModelInfo{{ID: modelID, Name: modelID, Provider: id}},
	}
	if !healthy {
		p.chatErr = ErrProviderUnhealthy
		p.streamErr = ErrProviderUnhealthy
	}
	return p
}

func (m *configurableMockProvider) ID() string { return m.id }
func (m *configurableMockProvider) Name() string { return m.name }
func (m *configurableMockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return m.models, nil
}
func NewMockProviderErr(msg string) error {
	return fmt.Errorf("mock: %s", msg)
}

func (m *configurableMockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, req)
	}
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return &ChatResponse{Model: m.modelID, Provider: m.id}, nil
}
func (m *configurableMockProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan StreamDelta, 1)
	ch <- StreamDelta{Done: true, Type: "done"}
	close(ch)
	return ch, nil
}
func (m *configurableMockProvider) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	return 0, nil
}
func (m *configurableMockProvider) ValidateConfig() error { return nil }
