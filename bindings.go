package main

import (
	"context"
	"fmt"

	"github.com/KleaSCM/nala/internal/agent"
	"github.com/KleaSCM/nala/internal/db"
	"github.com/KleaSCM/nala/internal/model"
)

func (a *App) CreateAgent(name, systemPrompt, personality string, params agentParams) (*agent.Agent, error) {
	ag := &agent.Agent{
		Name:         name,
		SystemPrompt: systemPrompt,
		Personality:  personality,
	}
	if params.MaxTokens > 0 {
		ag.MaxTokens = params.MaxTokens
	}
	if params.Temperature > 0 {
		ag.Temperature = params.Temperature
	}
	if params.TopP > 0 {
		ag.TopP = params.TopP
	}
	if params.TimeoutMS > 0 {
		ag.TimeoutMS = params.TimeoutMS
	}
	if params.MaxRetries > 0 {
		ag.MaxRetries = params.MaxRetries
	}
	if len(params.ToolIDs) > 0 {
		ag.ToolBindings = fmt.Sprintf(`[%s]`, joinQuoted(params.ToolIDs, ","))
	}
	if err := a.engine.AgentManager.Create(ag); err != nil {
		return nil, err
	}
	return ag, nil
}

func (a *App) GetAgent(id string) (*agent.Agent, error) {
	return a.engine.AgentManager.Get(id)
}

type listAgentsFilter struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Name   string `json:"name"`
}

func (a *App) ListAgents(filter listAgentsFilter) ([]*agent.Agent, error) {
	return a.engine.AgentManager.List(agent.AgentFilter{
		Limit:  filter.Limit,
		Offset: filter.Offset,
		Name:   filter.Name,
	})
}

func (a *App) UpdateAgent(ag *agent.Agent) error {
	return a.engine.AgentManager.Update(ag)
}

func (a *App) DeleteAgent(id string) error {
	return a.engine.AgentManager.Delete(id)
}

// ── Session ──────────────────────────────────────────────────────────

func (a *App) CreateSession(agentID string) (*agent.Session, error) {
	s := &agent.Session{
		AgentID: agentID,
	}
	if err := a.engine.SessionManager.Create(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (a *App) GetSession(id string) (*agent.Session, error) {
	return a.engine.SessionManager.Get(id)
}

type listSessionsFilter struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (a *App) ListSessions(filter listSessionsFilter) ([]*agent.Session, error) {
	return a.engine.SessionManager.List(agent.SessionFilter{
		AgentID: filter.AgentID,
		Status:  filter.Status,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
}

func (a *App) DeleteSession(id string) error {
	return a.engine.SessionManager.Delete(id)
}

func (a *App) PauseSession(id string) error {
	return a.engine.SessionManager.Pause(id)
}

func (a *App) ResumeSession(id string) error {
	return a.engine.SessionManager.Resume(id)
}

// ── Chat ─────────────────────────────────────────────────────────────

func (a *App) SendMessage(sessionID, message string) (*agent.TurnResult, error) {
	return a.engine.ConversationLoop.ProcessMessage(context.Background(), sessionID, message)
}

// ── Model / Provider ──────────────────────────────────────────────────

type providerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *App) ListProviders() []providerInfo {
	providers := a.engine.ModelRegistry.List()
	infos := make([]providerInfo, 0, len(providers))
	for _, p := range providers {
		infos = append(infos, providerInfo{ID: p.ID(), Name: p.Name()})
	}
	return infos
}

func (a *App) ListModels(providerID string) ([]model.ModelInfo, error) {
	p, err := a.engine.ModelRegistry.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.ListModels(context.Background())
}

// ── Settings ──────────────────────────────────────────────────────────

func (a *App) GetAppSetting(key string) (string, error) {
	return db.GetAppState(a.engine.DB, key)
}

func (a *App) SetAppSetting(key, value string) error {
	return db.SetAppState(a.engine.DB, key, value)
}

// ── Helpers ───────────────────────────────────────────────────────────

type agentParams struct {
	MaxTokens   int      `json:"max_tokens"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	TimeoutMS   int      `json:"timeout_ms"`
	MaxRetries  int      `json:"max_retries"`
	ToolIDs     []string `json:"tool_ids"`
}

func joinQuoted(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := `"` + items[0] + `"`
	for i := 1; i < len(items); i++ {
		result += sep + `"` + items[i] + `"`
	}
	return result
}
