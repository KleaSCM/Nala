package tool

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

type Info struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Schema      json.RawMessage `json:"schema"`
}

type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := t.ID()
	if _, exists := r.tools[id]; exists {
		return fmt.Errorf("tool: %q already registered", id)
	}
	r.tools[id] = t
	return nil
}

func (r *Registry) RegisterMany(tools ...Tool) error {
	for _, t := range tools {
		if err := r.Register(t); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Get(id string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, exists := r.tools[id]
	if !exists {
		return nil, fmt.Errorf("tool: %q not found", id)
	}
	return t, nil
}

func (r *Registry) List(category string) []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Info
	for _, t := range r.tools {
		if category != "" {
			cat := toolCategory(t.ID())
			if cat != category {
				continue
			}
		}
		result = append(result, Info{
			ID:          t.ID(),
			Name:        t.Name(),
			Description: t.Description(),
			Category:    toolCategory(t.ID()),
			Schema:      t.ParameterSchema(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (r *Registry) All() []Info {
	return r.List("")
}

func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	for _, t := range r.tools {
		cat := toolCategory(t.ID())
		seen[cat] = true
	}
	var cats []string
	for c := range seen {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats
}

func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, id)
}

func toolCategory(toolID string) string {
	for i := 0; i < len(toolID); i++ {
		if toolID[i] == '.' {
			return toolID[:i]
		}
	}
	return "uncategorized"
}
