/**
 * Provider interface for LLM model integration.
 * LLMモデル連携のプロバイダインターフェースね。
 *
 * Every provider (Ollama, OpenAI, Anthropic, etc.) implements this
 * interface. The registry and router use it polymorphically.
 * すべてのプロバイダ（Ollama、OpenAI、Anthropicなど）がこのインターフェースを
 * 実装するの。レジストリとルーターはポリモーフィックに使うわ。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"errors"
)

var (
	ErrProviderNotFound   = errors.New("model: provider not found")
	ErrModelNotFound      = errors.New("model: model not found in provider")
	ErrProviderUnhealthy  = errors.New("model: provider not healthy")
	ErrRateLimited        = errors.New("model: rate limited")
	ErrAuthentication     = errors.New("model: authentication failed")
	ErrContextTooLong     = errors.New("model: context too long")
	ErrStreamInterrupted  = errors.New("model: stream interrupted")
	ErrNoFallback         = errors.New("model: all fallbacks exhausted")
	ErrRefusalDetected    = errors.New("model: model refused to answer")
)

type Provider interface {
	ID() string
	Name() string
	ListModels(ctx context.Context) ([]ModelInfo, error)
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error)
	CountTokens(ctx context.Context, msgs []Message) (int, error)
	ValidateConfig() error
}

type Message struct {
	Role         string    `json:"role"`
	Content      string    `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	ToolResult   string    `json:"tool_result,omitempty"`
	ToolCallID   string    `json:"tool_call_id,omitempty"`
	Name         string    `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatRequest struct {
	Model        string      `json:"model"`
	Messages     []Message   `json:"messages"`
	SystemPrompt string      `json:"system_prompt,omitempty"`
	Parameters   Parameters  `json:"parameters,omitempty"`
	Tools        []ToolDef   `json:"tools,omitempty"`
	Stream       bool        `json:"stream"`
}

type ToolDef struct {
	Type       string          `json:"type"`
	Function   ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  map[string]any      `json:"parameters"`
}

type ChatResponse struct {
	Message   Message    `json:"message"`
	Usage     TokenUsage `json:"usage"`
	Model     string     `json:"model"`
	Duration  int64      `json:"duration_ms"`
	Provider  string     `json:"provider"`
}

type StreamDelta struct {
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Done       bool       `json:"done"`
	Usage      *TokenUsage `json:"usage,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type Parameters struct {
	Temperature      float64   `json:"temperature,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	Seed             int       `json:"seed,omitempty"`
}

type TokenUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ModelInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Provider   string   `json:"provider"`
	Tags       []string `json:"tags,omitempty"`
	ContextLen int      `json:"context_length"`
	MaxOutput  int      `json:"max_output,omitempty"`
	CostInput  float64  `json:"cost_per_input_token,omitempty"`
	CostOutput float64  `json:"cost_per_output_token,omitempty"`
}
