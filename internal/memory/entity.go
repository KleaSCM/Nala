/**
 * Entity extraction from text — regex + NLP-light for offline use.
 * テキストからエンティティ抽出 — オフラインでも使える軽量なの。
 *
 * Entity types: person, organization, project, event, date, location, email, phone, url, technology
 * エンティティタイプ: person, organization, project, event, date, location, email, phone, url, technology
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

type Entity struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	Context    string  `json:"context,omitempty"`
	Confidence float64 `json:"confidence"`
}

type EntityExtractor struct {
	mu       sync.RWMutex
	entities map[string]*EntityStore
}

type EntityStore struct {
	Type         string    `json:"type"`
	Value        string    `json:"value"`
	Contexts     []string  `json:"contexts"`
	Confidence   float64   `json:"confidence"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	SourceCount  int       `json:"source_count"`
}

var entityPatterns = map[string]*regexp.Regexp{
	"email":        regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
	"url":          regexp.MustCompile(`https?://[^\s<>"']+|www\.[^\s<>"']+`),
	"phone":        regexp.MustCompile(`\+?[\d\-\(\)\s]{7,20}`),
	"date_iso":     regexp.MustCompile(`\d{4}-\d{2}-\d{2}`),
	"date_common":  regexp.MustCompile(`\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]* \d{1,2},?\s?\d{4}\b`),
	"time":         regexp.MustCompile(`\b\d{1,2}:\d{2}\s?(AM|PM|am|pm)?\b`),
}

func NewEntityExtractor() *EntityExtractor {
	return &EntityExtractor{
		entities: make(map[string]*EntityStore),
	}
}

func (ee *EntityExtractor) Extract(text string) []Entity {
	var entities []Entity

	for etype, pattern := range entityPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, m := range matches {
			entities = append(entities, Entity{
				Type:       etype,
				Value:      m,
				Confidence: 0.8,
			})
		}
	}

	// Person detection: capitalized words following Mr/Ms/Dr or in list
	personRE := regexp.MustCompile(`\b(Mr\.|Ms\.|Mrs\.|Dr\.|Prof\.)\s+([A-Z][a-z]+)\b`)
	personMatches := personRE.FindAllStringSubmatch(text, -1)
	for _, m := range personMatches {
		entities = append(entities, Entity{
			Type:       "person",
			Value:      m[2],
			Context:    m[0],
			Confidence: 0.7,
		})
	}

	// Simple bare person name: "my name is X" or "I am X" or "called X"
	nameRE := regexp.MustCompile(`(?i)(?:my name is|i am|i'm|called|name is)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?)`)
	nameMatches := nameRE.FindAllStringSubmatch(text, -1)
	for _, m := range nameMatches {
		entities = append(entities, Entity{
			Type:       "person",
			Value:      m[1],
			Context:    m[0],
			Confidence: 0.6,
		})
	}

	// Technology detection: common tech terms
	techPatterns := []string{
		"Python", "JavaScript", "TypeScript", "Go", "Rust", "C\\+\\+", "Java",
		"React", "Vue", "Angular", "Node", "Docker", "Kubernetes", "SQL",
		"PostgreSQL", "SQLite", "Redis", "MongoDB", "GraphQL", "REST",
		"Linux", "Windows", "macOS", "AWS", "GCP", "Azure", "Git",
		"Ollama", "OpenAI", "Claude", "Gemini", "LLaMA", "Mistral",
		"Wails", "Svelte", "Tailwind", "Vite",
	}
	for _, tech := range techPatterns {
		re := regexp.MustCompile(`\b` + tech + `\b`)
		if re.MatchString(text) {
			entities = append(entities, Entity{
				Type:       "technology",
				Value:      tech,
				Confidence: 0.9,
			})
		}
	}

	// Organization: @domain → extract org name
	orgRE := regexp.MustCompile(`@([A-Z][a-zA-Z]+)\.`)
	orgMatches := orgRE.FindAllStringSubmatch(text, -1)
	for _, m := range orgMatches {
		entities = append(entities, Entity{
			Type:       "organization",
			Value:      m[1],
			Confidence: 0.5,
		})
	}

	return ee.deduplicate(entities)
}

func (ee *EntityExtractor) ExtractFromConversation(messages []string) []Entity {
	var all []Entity
	for _, msg := range messages {
		all = append(all, ee.Extract(msg)...)
	}
	return ee.deduplicate(all)
}

func (ee *EntityExtractor) Store(text string) {
	entities := ee.Extract(text)
	ee.mu.Lock()
	defer ee.mu.Unlock()

	now := time.Now()
	for _, e := range entities {
		key := e.Type + ":" + e.Value
		store, ok := ee.entities[key]
		if !ok {
			ee.entities[key] = &EntityStore{
				Type:        e.Type,
				Value:       e.Value,
				Contexts:    []string{e.Context},
				Confidence:  e.Confidence,
				FirstSeen:   now,
				LastSeen:    now,
				SourceCount: 1,
			}
		} else {
			store.LastSeen = now
			store.SourceCount++
			// Boost confidence on re-appearance
			if store.Confidence < 1.0 {
				store.Confidence = min(1.0, store.Confidence+0.1)
			}
			// Add context if new
			if e.Context != "" {
				exists := false
				for _, c := range store.Contexts {
					if c == e.Context {
						exists = true
						break
					}
				}
				if !exists {
					store.Contexts = append(store.Contexts, e.Context)
				}
			}
		}
	}
}

func (ee *EntityExtractor) Find(value string) []*EntityStore {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	var results []*EntityStore
	for _, store := range ee.entities {
		if strings.Contains(strings.ToLower(store.Value), strings.ToLower(value)) {
			results = append(results, store)
		}
	}
	return results
}

func (ee *EntityExtractor) FindByType(etype string) []*EntityStore {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	var results []*EntityStore
	for _, store := range ee.entities {
		if store.Type == etype {
			results = append(results, store)
		}
	}
	return results
}

func (ee *EntityExtractor) All() []*EntityStore {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	results := make([]*EntityStore, 0, len(ee.entities))
	for _, store := range ee.entities {
		results = append(results, store)
	}
	return results
}

func (ee *EntityExtractor) deduplicate(entities []Entity) []Entity {
	seen := make(map[string]bool)
	var result []Entity
	for _, e := range entities {
		key := e.Type + ":" + e.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, e)
	}
	return result
}

func (ee *EntityExtractor) Count() int {
	ee.mu.RLock()
	defer ee.mu.RUnlock()
	return len(ee.entities)
}
