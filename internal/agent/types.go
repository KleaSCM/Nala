/**
 * Core domain types for the agent subsystem.
 * エージェントサブシステムのコアドメインタイプね。
 *
 * Defines Agent, Session, Message, ToolBinding, and related value types
 * with validation. These map 1:1 to the SQL tables in migration 001-004.
 * Agent、Session、Message、ToolBinding、および関連する値タイプを定義してるの。
 * マイグレーション001〜004のSQLテーブルと1:1対応してるわ。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package agent

import (
	"fmt"
	"regexp"
	"time"
)

type Agent struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Description     string `json:"description,omitempty"`
	SystemPrompt    string `json:"system_prompt,omitempty"`
	ModelConfig     string `json:"model_config"`
	ToolBindings    string `json:"tool_bindings"`
	MemoryConfig    string `json:"memory_config"`
	MaxTokens       int    `json:"max_tokens"`
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"top_p"`
	PresencePenalty float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	Personality     string `json:"personality"`
	TimeoutMS       int    `json:"timeout_ms"`
	MaxRetries      int    `json:"max_retries"`
	Metadata        string `json:"metadata"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$`)

func (a *Agent) Validate() error {
	if a.Name == "" || len(a.Name) > 100 {
		return fmt.Errorf("agent: name must be 1-100 characters")
	}
	if !slugRe.MatchString(a.Slug) {
		return fmt.Errorf("agent: slug must match ^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$")
	}
	if a.Temperature < 0 || a.Temperature > 2 {
		return fmt.Errorf("agent: temperature must be in [0.0, 2.0]")
	}
	if a.TopP < 0 || a.TopP > 1 {
		return fmt.Errorf("agent: top_p must be in [0.0, 1.0]")
	}
	if a.TimeoutMS < 1000 || a.TimeoutMS > 600000 {
		return fmt.Errorf("agent: timeout_ms must be in [1000, 600000]")
	}
	if a.MaxRetries < 0 || a.MaxRetries > 10 {
		return fmt.Errorf("agent: max_retries must be in [0, 10]")
	}
	return nil
}

type Session struct {
	ID             string `json:"id"`
	AgentID        string `json:"agent_id"`
	Title          string `json:"title,omitempty"`
	Status         string `json:"status"`
	ContextSize    int    `json:"context_size"`
	MaxContextSize int    `json:"max_context_size"`
	MessageCount   int    `json:"message_count"`
	TotalTokensIn  int    `json:"total_tokens_in"`
	TotalTokensOut int    `json:"total_tokens_out"`
	TotalCost      float64 `json:"total_cost"`
	Metadata       string `json:"metadata"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	PausedAt       *string `json:"paused_at,omitempty"`
	CompletedAt    *string `json:"completed_at,omitempty"`
}

func (s *Session) Validate() error {
	validStatuses := map[string]bool{"active": true, "paused": true, "completed": true, "archived": true}
	if !validStatuses[s.Status] {
		return fmt.Errorf("session: invalid status %q", s.Status)
	}
	if s.MaxContextSize < 1024 || s.MaxContextSize > 1048576 {
		return fmt.Errorf("session: max_context_size must be 1024-1048576")
	}
	return nil
}

type Message struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ParentID    *string `json:"parent_id,omitempty"`
	Role        string `json:"role"`
	Content     *string `json:"content,omitempty"`
	ToolCalls   string `json:"tool_calls"`
	ToolResults string `json:"tool_results"`
	Model       *string `json:"model,omitempty"`
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
	DurationMS  *int   `json:"duration_ms,omitempty"`
	Error       *string `json:"error,omitempty"`
	Metadata    string `json:"metadata"`
	CreatedAt   string `json:"created_at"`
}

func (m *Message) Validate() error {
	validRoles := map[string]bool{"user": true, "assistant": true, "system": true, "tool": true}
	if !validRoles[m.Role] {
		return fmt.Errorf("message: invalid role %q", m.Role)
	}
	return nil
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

type ToolBinding struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	ToolID    string `json:"tool_id"`
	Config    string `json:"config"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func (tb *ToolBinding) Validate() error {
	if tb.ToolID == "" {
		return fmt.Errorf("tool_binding: tool_id is required")
	}
	return nil
}

type ModelConfig struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	MaxTokens        int     `json:"max_tokens"`
	Temperature      float64 `json:"temperature"`
	TopP             float64 `json:"top_p"`
	PresencePenalty  float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	Stop             []string `json:"stop,omitempty"`
}

type MemoryConfig struct {
	VectorBackend            string `json:"vector_backend"`
	DefaultChunkSize         int    `json:"default_chunk_size"`
	DefaultChunkOverlap      int    `json:"default_chunk_overlap"`
	SummarizationIntervalMin int    `json:"summarization_interval_min"`
	AutoExtractFacts         bool   `json:"auto_extract_facts"`
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
