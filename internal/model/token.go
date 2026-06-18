/**
 * Token usage tracking and cost calculation.
 * トークン使用量の追跡とコスト計算ね。
 *
 * Tracks per-message and per-session token usage, provides cost lookup
 * per provider, and accumulates totals for the usage dashboard.
 * メッセージごと・セッションごとのトークン使用量を追跡して、
 * プロバイダごとのコスト計算と使用量ダッシュボード用の集計を提供するの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"math"
	"sync"
)

type CostTable struct {
	mu       sync.RWMutex
	providers map[string]map[string][2]float64
}

func NewCostTable() *CostTable {
	ct := &CostTable{
		providers: make(map[string]map[string][2]float64),
	}

	ct.SetCost("openai", "gpt-4o", 2.50/1000, 10.00/1000)
	ct.SetCost("openai", "gpt-4o-mini", 0.15/1000, 0.60/1000)
	ct.SetCost("openai", "o1", 15.00/1000, 60.00/1000)
	ct.SetCost("openai", "o3-mini", 1.10/1000, 4.40/1000)
	ct.SetCost("openai", "*", 0.15/1000, 0.60/1000)

	ct.SetCost("anthropic", "claude-3.5-sonnet", 3.00/1000, 15.00/1000)
	ct.SetCost("anthropic", "claude-4-opus", 15.00/1000, 75.00/1000)
	ct.SetCost("anthropic", "*", 3.00/1000, 15.00/1000)

	ct.SetCost("ollama", "*", 0, 0)
	ct.SetCost("llama.cpp", "*", 0, 0)

	return ct
}

func (ct *CostTable) SetCost(provider, model string, inputCost, outputCost float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if ct.providers[provider] == nil {
		ct.providers[provider] = make(map[string][2]float64)
	}
	ct.providers[provider][model] = [2]float64{inputCost, outputCost}
}

func (ct *CostTable) CostFor(provider, model string, inputTokens, outputTokens int) float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	models, ok := ct.providers[provider]
	if !ok {
		return 0
	}

	costs, exact := models[model]
	if exact {
		return math.Round((costs[0]*float64(inputTokens)+costs[1]*float64(outputTokens))*100000) / 100000
	}

	costs, wildcard := models["*"]
	if wildcard {
		return math.Round((costs[0]*float64(inputTokens)+costs[1]*float64(outputTokens))*100000) / 100000
	}

	return 0
}

type TokenTracker struct {
	mu             sync.Mutex
	SessionTokens  map[string]*SessionTokenUsage
	GlobalTokens   *SessionTokenUsage
	costTable      *CostTable
}

type SessionTokenUsage struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
	MessageCount     int     `json:"message_count"`
}

func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		SessionTokens: make(map[string]*SessionTokenUsage),
		GlobalTokens:  &SessionTokenUsage{},
		costTable:     NewCostTable(),
	}
}

func (tt *TokenTracker) CostTable() *CostTable {
	return tt.costTable
}

func (tt *TokenTracker) TrackMessage(sessionID, provider, model string, usage TokenUsage) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	cost := tt.costTable.CostFor(provider, model, usage.InputTokens, usage.OutputTokens)

	if _, exists := tt.SessionTokens[sessionID]; !exists {
		tt.SessionTokens[sessionID] = &SessionTokenUsage{}
	}
	su := tt.SessionTokens[sessionID]
	su.InputTokens += usage.InputTokens
	su.OutputTokens += usage.OutputTokens
	su.TotalTokens += usage.TotalTokens
	su.TotalCost += cost
	su.MessageCount++

	tt.GlobalTokens.InputTokens += usage.InputTokens
	tt.GlobalTokens.OutputTokens += usage.OutputTokens
	tt.GlobalTokens.TotalTokens += usage.TotalTokens
	tt.GlobalTokens.TotalCost += cost
	tt.GlobalTokens.MessageCount++
}

func (tt *TokenTracker) SessionUsage(sessionID string) *SessionTokenUsage {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	usage, exists := tt.SessionTokens[sessionID]
	if !exists {
		return &SessionTokenUsage{}
	}
	cp := *usage
	return &cp
}

func (tt *TokenTracker) GlobalUsage() *SessionTokenUsage {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	cp := *tt.GlobalTokens
	return &cp
}

func (tt *TokenTracker) ResetSession(sessionID string) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if usage, exists := tt.SessionTokens[sessionID]; exists {
		tt.GlobalTokens.InputTokens -= usage.InputTokens
		tt.GlobalTokens.OutputTokens -= usage.OutputTokens
		tt.GlobalTokens.TotalTokens -= usage.TotalTokens
		tt.GlobalTokens.TotalCost -= usage.TotalCost
		tt.GlobalTokens.MessageCount -= usage.MessageCount
		delete(tt.SessionTokens, sessionID)
	}
}

func (tt *TokenTracker) ResetAll() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.SessionTokens = make(map[string]*SessionTokenUsage)
	tt.GlobalTokens = &SessionTokenUsage{}
}
