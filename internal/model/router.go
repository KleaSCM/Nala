/**
 * Multi-model router — routes chat requests to the best provider/model.
 * マルチモデルルーター — チャットリクエストを最適なプロバイダ/モデルにルーティングするの。
 *
 * Uses intent detection (keyword analysis) to route queries: code→code model,
 * chat→local, reasoning→cloud. Supports per-agent routing rules, priority
 * ordering, explicit model overrides, and fallback chains.
 * インテント検出（キーワード分析）でクエリを振り分けるの。コード→コードモデル、
 * チャット→ローカル、推論→クラウド。エージェントごとのルール、優先順位、
 * 明示的なモデルオーバーライド、フォールバックチェーンに対応してるわ。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"fmt"
	"strings"
)

type Intent string

const (
	IntentChat      Intent = "chat"
	IntentCode      Intent = "code"
	IntentReasoning Intent = "reasoning"
	IntentCreative  Intent = "creative"
	IntentTool      Intent = "tool"
	IntentSecurity  Intent = "security"
	IntentUnknown   Intent = "unknown"
)

type RoutingRule struct {
	Intent   Intent
	Provider string
	Model    string
	Priority int
}

type Router struct {
	registry *Registry
	rules    []RoutingRule
}

func NewRouter(registry *Registry) *Router {
	return &Router{
		registry: registry,
		rules:    defaultRoutingRules(),
	}
}

func defaultRoutingRules() []RoutingRule {
	return []RoutingRule{
		{Intent: IntentCode, Provider: "ollama", Model: "qwen2.5-coder:1.5b", Priority: 10},
		{Intent: IntentCode, Provider: "openai", Model: "gpt-4o-mini", Priority: 5},
		{Intent: IntentReasoning, Provider: "openai", Model: "o3-mini", Priority: 10},
		{Intent: IntentReasoning, Provider: "ollama", Model: "qwen3.5:9b", Priority: 5},
		{Intent: IntentCreative, Provider: "ollama", Model: "dolphin-phi:latest", Priority: 10},
		{Intent: IntentSecurity, Provider: "ollama", Model: "gemma-4-heretic", Priority: 10},
		{Intent: IntentChat, Provider: "ollama", Model: "qwen3.5:9b", Priority: 10},
		{Intent: IntentChat, Provider: "openai", Model: "gpt-4o-mini", Priority: 5},
		{Intent: IntentTool, Provider: "ollama", Model: "mistral-nemo:latest", Priority: 10},
		{Intent: IntentUnknown, Provider: "ollama", Model: "qwen3.5:9b", Priority: 10},
	}
}

var intentKeywords = map[Intent][]string{
	IntentCode:      {"write code", "function", "implement", "program", "debug", "fix bug", "algorithm",
		"refactor", "compile", "syntax", "api call", "script", "bash", "python", "golang", "rust",
		"typescript", "javascript", "sql query", "regex", "unit test"},
	IntentReasoning: {"explain", "why", "how does", "compare", "analyze", "what if", "reason",
		"step by step", "think through", "logical", "cause", "argument", "prove", "conclusion"},
	IntentCreative:  {"write a story", "poem", "creative", "imagine", "invent", "design", "brainstorm",
		"generate ideas", "roleplay", "scenario", "worldbuilding", "character"},
	IntentSecurity:  {"hack", "exploit", "vulnerability", "penetration test", "crack", "bypass",
		"privilege escalation", "buffer overflow", "injection", "payload", "reverse shell",
		"malware", "ransomware", "rootkit", "cve", "0day"},
	IntentTool:      {"run tool", "execute", "shell command", "terminal", "run ", "list files",
		"search for", "find file", "read file", "edit file"},
}

func DetectIntent(message string) Intent {
	lower := strings.ToLower(message)
	bestIntent := IntentChat
	bestScore := 0

	for intent, keywords := range intentKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	return bestIntent
}

func (r *Router) SetRules(rules []RoutingRule) {
	r.rules = rules
}

func (r *Router) AddRule(rule RoutingRule) {
	r.rules = append(r.rules, rule)
}

func (r *Router) Route(ctx context.Context, req ChatRequest) (string, string, error) {
	if req.Model != "" {
		_, err := r.registry.Get(req.Model)
		if err == nil {
			return req.Model, req.Model, nil
		}
		for _, p := range r.registry.List() {
			models, listErr := p.ListModels(ctx)
			if listErr != nil {
				continue
			}
			for _, m := range models {
				if m.ID == req.Model || strings.HasPrefix(m.ID, req.Model) {
					return p.ID(), m.ID, nil
				}
			}
		}
	}

	intent := DetectIntent(req.SystemPrompt + "\n" + messagesText(req.Messages))

	var matchedRules []RoutingRule
	for _, rule := range r.rules {
		if rule.Intent == intent || rule.Intent == IntentUnknown {
			matchedRules = append(matchedRules, rule)
		}
	}

	for _, rule := range matchedRules {
		if _, err := r.registry.Get(rule.Provider); err == nil {
			return rule.Provider, rule.Model, nil
		}
	}

	for _, rule := range r.rules {
		if rule.Intent == IntentUnknown {
			if _, err := r.registry.Get(rule.Provider); err == nil {
				return rule.Provider, rule.Model, nil
			}
		}
	}

	return "", "", fmt.Errorf("router: no suitable provider found: %w", ErrProviderNotFound)
}

func (r *Router) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	providerID, modelName, err := r.Route(ctx, req)
	if err != nil {
		return nil, err
	}
	req.Model = modelName
	return r.registry.Chat(ctx, providerID, req)
}

func (r *Router) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	providerID, modelName, err := r.Route(ctx, req)
	if err != nil {
		return nil, err
	}
	req.Model = modelName
	return r.registry.ChatStream(ctx, providerID, req)
}

func messagesText(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteString(" ")
	}
	return b.String()
}
