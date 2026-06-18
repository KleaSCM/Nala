package agent

import (
	"fmt"
	"strings"
	"time"
)

type TemplateVars struct {
	Date           string
	Time           string
	DateTime       string
	UserName       string
	Tools          string
	ToolCount      int
	KnowledgeBases string
	SessionID      string
	AgentName      string
	Custom         map[string]string
}

func RenderSystemPrompt(tmpl string, vars TemplateVars) string {
	replacer := strings.NewReplacer(
		"{{date}}", vars.Date,
		"{{time}}", vars.Time,
		"{{datetime}}", vars.DateTime,
		"{{user_name}}", vars.UserName,
		"{{tools}}", vars.Tools,
		"{{tool_count}}", fmt.Sprintf("%d", vars.ToolCount),
		"{{knowledge_bases}}", vars.KnowledgeBases,
		"{{session_id}}", vars.SessionID,
		"{{agent_name}}", vars.AgentName,
	)
	result := replacer.Replace(tmpl)
	for key, val := range vars.Custom {
		result = strings.ReplaceAll(result, "{{"+key+"}}", val)
	}
	return result
}

func DefaultTemplateVars() TemplateVars {
	now := time.Now()
	return TemplateVars{
		Date:           now.Format("2006-01-02"),
		Time:           now.Format("15:04:05"),
		DateTime:       now.Format("2006-01-02 15:04:05 MST"),
		UserName:       "User",
		Tools:          "",
		KnowledgeBases: "",
		SessionID:      "",
		AgentName:      "Nala",
		Custom:         make(map[string]string),
	}
}

type PersonalityPreset struct {
	Name        string
	SystemPrompt string
}

var BuiltinPresets = map[string]PersonalityPreset{
	"default": {
		Name:        "default",
		SystemPrompt: "You are Nala, a helpful AI assistant. Be concise but thorough in your responses.",
	},
	"helpful": {
		Name:        "helpful",
		SystemPrompt: "You are Nala, a helpful AI assistant. Be concise but thorough. Focus on providing practical, actionable answers.",
	},
	"creative": {
		Name:        "creative",
		SystemPrompt: "You are Nala, a creative AI assistant. Be imaginative and expressive. Use vivid language and explore ideas freely.",
	},
	"precise": {
		Name:        "precise",
		SystemPrompt: "You are Nala, a precise AI assistant. Be accurate and factual. When uncertain, clearly state your confidence level.",
	},
	"code": {
		Name: "code",
		SystemPrompt: `You are Nala, an expert software engineer. Write clean, idiomatic, well-structured code.
Always consider edge cases, performance, and security. Explain your reasoning briefly.`,
	},
	"socratic": {
		Name: "socratic",
		SystemPrompt: `You are Nala, a Socratic guide. Don't give direct answers — ask guiding questions
that help the user discover the answer themselves. Encourage critical thinking.`,
	},
}

func ResolvePreset(presetName string) PersonalityPreset {
	p, exists := BuiltinPresets[presetName]
	if !exists {
		return BuiltinPresets["default"]
	}
	return p
}
