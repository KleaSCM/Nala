package agent

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

func (m *Manager) DB() *sql.DB { return m.db }

func nextSlug(db *sql.DB, base string) (string, error) {
	slug := base
	for i := 1; ; i++ {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM agents WHERE slug = ?", slug).Scan(&count)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (m *Manager) Create(a *Agent) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Slug == "" {
		a.Slug = strings.ToLower(strings.ReplaceAll(a.Name, " ", "-"))
	}
	if !slugRe.MatchString(a.Slug) {
		a.Slug = fmt.Sprintf("agent-%s", uuid.New().String()[:8])
	}
	s, err := nextSlug(m.db, a.Slug)
	if err != nil {
		return fmt.Errorf("agent: slug generation: %w", err)
	}
	a.Slug = s
	now := Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.MaxTokens == 0 {
		a.MaxTokens = 4096
	}
	if a.Temperature == 0 {
		a.Temperature = 0.7
	}
	if a.TopP == 0 {
		a.TopP = 0.9
	}
	if a.TimeoutMS == 0 {
		a.TimeoutMS = 300000
	}
	if a.MaxRetries == 0 {
		a.MaxRetries = 3
	}
	if a.ModelConfig == "" {
		a.ModelConfig = "{}"
	}
	if a.ToolBindings == "" {
		a.ToolBindings = "[]"
	}
	if a.MemoryConfig == "" {
		a.MemoryConfig = "{}"
	}
	if a.Metadata == "" {
		a.Metadata = "{}"
	}
	if a.Personality == "" {
		a.Personality = "default"
	}
	if err := a.Validate(); err != nil {
		return err
	}
	_, err = m.db.Exec(`INSERT INTO agents
		(id, name, slug, description, system_prompt, model_config, tool_bindings,
		 memory_config, max_tokens, temperature, top_p, presence_penalty,
		 frequency_penalty, personality, timeout_ms, max_retries, metadata,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Slug, a.Description, a.SystemPrompt,
		a.ModelConfig, a.ToolBindings, a.MemoryConfig,
		a.MaxTokens, a.Temperature, a.TopP, a.PresencePenalty,
		a.FrequencyPenalty, a.Personality, a.TimeoutMS, a.MaxRetries,
		a.Metadata, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("agent: insert failed: %w", err)
	}
	return nil
}

func (m *Manager) Get(id string) (*Agent, error) {
	a := &Agent{}
	err := m.db.QueryRow(`SELECT id, name, slug, description, system_prompt,
		model_config, tool_bindings, memory_config, max_tokens, temperature,
		top_p, presence_penalty, frequency_penalty, personality, timeout_ms,
		max_retries, metadata, created_at, updated_at
		FROM agents WHERE id = ?`, id).Scan(
		&a.ID, &a.Name, &a.Slug, &a.Description, &a.SystemPrompt,
		&a.ModelConfig, &a.ToolBindings, &a.MemoryConfig,
		&a.MaxTokens, &a.Temperature, &a.TopP, &a.PresencePenalty,
		&a.FrequencyPenalty, &a.Personality, &a.TimeoutMS, &a.MaxRetries,
		&a.Metadata, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("agent: get failed: %w", err)
	}
	return a, nil
}

func (m *Manager) GetBySlug(slug string) (*Agent, error) {
	a := &Agent{}
	err := m.db.QueryRow(`SELECT id, name, slug, description, system_prompt,
		model_config, tool_bindings, memory_config, max_tokens, temperature,
		top_p, presence_penalty, frequency_penalty, personality, timeout_ms,
		max_retries, metadata, created_at, updated_at
		FROM agents WHERE slug = ?`, slug).Scan(
		&a.ID, &a.Name, &a.Slug, &a.Description, &a.SystemPrompt,
		&a.ModelConfig, &a.ToolBindings, &a.MemoryConfig,
		&a.MaxTokens, &a.Temperature, &a.TopP, &a.PresencePenalty,
		&a.FrequencyPenalty, &a.Personality, &a.TimeoutMS, &a.MaxRetries,
		&a.Metadata, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("agent: get by slug failed: %w", err)
	}
	return a, nil
}

func (m *Manager) Update(a *Agent) error {
	if a.ID == "" {
		return fmt.Errorf("agent: id is required for update")
	}
	a.UpdatedAt = Now()
	if err := a.Validate(); err != nil {
		return err
	}
	res, err := m.db.Exec(`UPDATE agents SET
		name=?, slug=?, description=?, system_prompt=?, model_config=?,
		tool_bindings=?, memory_config=?, max_tokens=?, temperature=?,
		top_p=?, presence_penalty=?, frequency_penalty=?, personality=?,
		timeout_ms=?, max_retries=?, metadata=?, updated_at=?
		WHERE id=?`,
		a.Name, a.Slug, a.Description, a.SystemPrompt, a.ModelConfig,
		a.ToolBindings, a.MemoryConfig, a.MaxTokens, a.Temperature,
		a.TopP, a.PresencePenalty, a.FrequencyPenalty, a.Personality,
		a.TimeoutMS, a.MaxRetries, a.Metadata, a.UpdatedAt, a.ID)
	if err != nil {
		return fmt.Errorf("agent: update failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agent: not found")
	}
	return nil
}

func (m *Manager) Delete(id string) error {
	res, err := m.db.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("agent: delete failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agent: not found")
	}
	return nil
}

type AgentFilter struct {
	Limit  int
	Offset int
	Name   string
}

func (m *Manager) List(filter AgentFilter) ([]*Agent, error) {
	query := `SELECT id, name, slug, description, system_prompt,
		model_config, tool_bindings, memory_config, max_tokens, temperature,
		top_p, presence_penalty, frequency_penalty, personality, timeout_ms,
		max_retries, metadata, created_at, updated_at
		FROM agents`
	var args []any
	var conditions []string
	if filter.Name != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	} else {
		query += " LIMIT 100"
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}
	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("agent: list failed: %w", err)
	}
	defer rows.Close()
	var agents []*Agent
	for rows.Next() {
		a := &Agent{}
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Slug, &a.Description, &a.SystemPrompt,
			&a.ModelConfig, &a.ToolBindings, &a.MemoryConfig,
			&a.MaxTokens, &a.Temperature, &a.TopP, &a.PresencePenalty,
			&a.FrequencyPenalty, &a.Personality, &a.TimeoutMS, &a.MaxRetries,
			&a.Metadata, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("agent: scan failed: %w", err)
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []*Agent{}
	}
	return agents, rows.Err()
}
