package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/KleaSCM/nala/internal/model"
	"github.com/KleaSCM/nala/internal/tool"
	"github.com/google/uuid"
)

type ConversationLoop struct {
	manager      *Manager
	sessions     *SessionManager
	registry     *model.Registry
	router       *model.Router
	toolReg      *tool.Registry
	tokenTracker *model.TokenTracker
}

func (cl *ConversationLoop) Sessions() *SessionManager {
	return cl.sessions
}

func NewConversationLoop(
	mgr *Manager,
	sm *SessionManager,
	reg *model.Registry,
	router *model.Router,
	tr *tool.Registry,
	tt *model.TokenTracker,
) *ConversationLoop {
	return &ConversationLoop{
		manager:      mgr,
		sessions:     sm,
		registry:     reg,
		router:       router,
		toolReg:      tr,
		tokenTracker: tt,
	}
}

type TurnResult struct {
	Message  string            `json:"message"`
	Usage    model.TokenUsage  `json:"usage"`
	Model    string            `json:"model"`
	Provider string            `json:"provider"`
	Duration int64             `json:"duration_ms"`
	Error    string            `json:"error,omitempty"`
}

func (cl *ConversationLoop) ProcessMessage(ctx context.Context, sessionID, userMessage string) (*TurnResult, error) {
	session, err := cl.sessions.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("loop: session not found: %w", err)
	}

	agent, err := cl.manager.Get(session.AgentID)
	if err != nil {
		return nil, fmt.Errorf("loop: agent not found: %w", err)
	}

	if !cl.sessions.RateLimitAllow(agent.ID, 30, time.Minute) {
		return nil, fmt.Errorf("loop: rate limit exceeded")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(agent.TimeoutMS)*time.Millisecond)
	defer cancel()

	if session.Title == "" {
		title := autoTitle(userMessage)
		cl.sessions.SetTitle(sessionID, title)
	}

	history, err := cl.loadHistory(sessionID)
	if err != nil {
		return nil, fmt.Errorf("loop: load history: %w", err)
	}

	userMsg := model.Message{Role: "user", Content: userMessage}
	history = append(history, userMsg)

	tools := cl.buildToolDefs(agent.ID)

	systemPrompt := cl.buildSystemPrompt(agent, sessionID)

	req := model.ChatRequest{
		Model:        "",
		Messages:     history,
		SystemPrompt: systemPrompt,
		Parameters: model.Parameters{
			Temperature:      agent.Temperature,
			TopP:             agent.TopP,
			MaxTokens:        agent.MaxTokens,
		},
		Tools: tools,
	}

	for iter := 0; iter < 20; iter++ {
		start := time.Now()
		resp, err := cl.router.Chat(ctx, req)
		duration := time.Since(start).Milliseconds()

		if err != nil {
			return &TurnResult{
				Error:    err.Error(),
				Duration: duration,
			}, nil
		}

		usage := resp.Usage
		cl.tokenTracker.TrackMessage(sessionID, resp.Provider, resp.Model, usage)
		cl.sessions.TrackUsage(sessionID, usage.InputTokens, usage.OutputTokens,
			cl.tokenTracker.CostTable().CostFor(resp.Provider, resp.Model, usage.InputTokens, usage.OutputTokens))

		if len(resp.Message.ToolCalls) == 0 {
		result := &TurnResult{
			Message:  resp.Message.Content,
			Usage:    usage,
			Model:    resp.Model,
			Provider: resp.Provider,
			Duration: duration,
		}
		cl.saveMessages(sessionID, userMsg, model.Message{
			Role:    "assistant",
			Content: resp.Message.Content,
			Name:    resp.Model,
		})
		return result, nil
		}

		toolResults := cl.executeToolCalls(ctx, resp.Message.ToolCalls, agent.ID)
		req.Messages = append(req.Messages, resp.Message)
		req.Messages = append(req.Messages, toolResults...)
	}

	return &TurnResult{Error: "loop: max iterations reached"}, nil
}

func (cl *ConversationLoop) executeToolCalls(ctx context.Context, calls []model.ToolCall, agentID string) []model.Message {
	var results []model.Message
	for _, tc := range calls {
		t, err := cl.toolReg.Get(tc.Function.Name)
		if err != nil {
			results = append(results, model.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				ToolResult: fmt.Sprintf("tool %q not found", tc.Function.Name),
			})
			continue
		}
		args := json.RawMessage(tc.Function.Arguments)
		start := time.Now()
		result, execErr := t.Execute(ctx, args)
		_ = time.Since(start).Milliseconds()
		if execErr != nil {
			results = append(results, model.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				ToolResult: execErr.Error(),
			})
			continue
		}
		content := ""
		if result != nil {
			content = result.Content
		}
		results = append(results, model.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			ToolResult: content,
		})
	}
	return results
}

func (cl *ConversationLoop) loadHistory(sessionID string) ([]model.Message, error) {
	rows, err := cl.manager.DB().Query(
		`SELECT role, content, tool_calls, tool_results, model
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		var content, toolCalls, toolResults *string
		var modelStr *string
		if err := rows.Scan(&m.Role, &content, &toolCalls, &toolResults, &modelStr); err != nil {
			return nil, err
		}
		if content != nil {
			m.Content = *content
		}
		if toolCalls != nil && *toolCalls != "[]" {
			json.Unmarshal([]byte(*toolCalls), &m.ToolCalls)
		}
		if toolResults != nil && *toolResults != "[]" {
			m.ToolResult = *toolResults
		}
		if modelStr != nil {
			m.Name = *modelStr
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (cl *ConversationLoop) saveMessages(sessionID string, msgs ...model.Message) {
	for _, m := range msgs {
		id := uuid.New().String()
		content := &m.Content
		if m.Content == "" {
			content = nil
		}
		tcJSON, _ := json.Marshal(m.ToolCalls)
		tc := string(tcJSON)
		now := Now()

		cl.manager.DB().Exec(`INSERT INTO messages
			(id, session_id, role, content, tool_calls, tool_results, model,
			 tokens_in, tokens_out, duration_ms, error, metadata, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, sessionID, m.Role, content, tc, m.ToolResult, m.Name,
			0, 0, 0, nil, "{}", now)
	}
}

func (cl *ConversationLoop) buildToolDefs(agentID string) []model.ToolDef {
	tools := cl.toolReg.All()
	defs := make([]model.ToolDef, 0, len(tools))
	for _, info := range tools {
		defs = append(defs, model.ToolDef{
			Type: "function",
			Function: model.ToolFunctionDef{
				Name:        info.ID,
				Description: info.Description,
				Parameters:  nil,
			},
		})
	}
	return defs
}

func (cl *ConversationLoop) buildSystemPrompt(agent *Agent, sessionID string) string {
	vars := DefaultTemplateVars()
	vars.AgentName = agent.Name
	vars.SessionID = sessionID
	vars.ToolCount = len(cl.toolReg.All())
	vars.Tools = cl.toolNames()
	prompt := agent.SystemPrompt
	if prompt == "" {
		p := ResolvePreset(agent.Personality)
		prompt = p.SystemPrompt
	}
	return RenderSystemPrompt(prompt, vars)
}

func (cl *ConversationLoop) toolNames() string {
	infos := cl.toolReg.All()
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.ID
	}
	return strings.Join(names, ", ")
}

func autoTitle(msg string) string {
	if len(msg) > 60 {
		return msg[:60] + "..."
	}
	return msg
}
