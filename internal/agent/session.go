package agent

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionManager struct {
	db          *sql.DB
	mu          sync.RWMutex
	sessions    map[string]context.CancelFunc
	rateLimiters map[string]*rateLimiter
}

func NewSessionManager(db *sql.DB) *SessionManager {
	return &SessionManager{
		db:          db,
		sessions:    make(map[string]context.CancelFunc),
		rateLimiters: make(map[string]*rateLimiter),
	}
}

func (sm *SessionManager) Create(s *Session) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = "active"
	}
	if s.MaxContextSize == 0 {
		s.MaxContextSize = 128000
	}
	if s.Metadata == "" {
		s.Metadata = "{}"
	}
	now := Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	if err := s.Validate(); err != nil {
		return err
	}
	_, err := sm.db.Exec(`INSERT INTO sessions
		(id, agent_id, title, status, context_size, max_context_size,
		 message_count, total_tokens_in, total_tokens_out, total_cost,
		 metadata, created_at, updated_at, paused_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.AgentID, s.Title, s.Status, s.ContextSize, s.MaxContextSize,
		s.MessageCount, s.TotalTokensIn, s.TotalTokensOut, s.TotalCost,
		s.Metadata, s.CreatedAt, s.UpdatedAt, s.PausedAt, s.CompletedAt)
	if err != nil {
		return fmt.Errorf("session: create failed: %w", err)
	}
	return nil
}

func (sm *SessionManager) Get(id string) (*Session, error) {
	s := &Session{}
	var pausedAt, completedAt sql.NullString
	err := sm.db.QueryRow(`SELECT id, agent_id, title, status, context_size,
		max_context_size, message_count, total_tokens_in, total_tokens_out,
		total_cost, metadata, created_at, updated_at, paused_at, completed_at
		FROM sessions WHERE id = ?`, id).Scan(
		&s.ID, &s.AgentID, &s.Title, &s.Status, &s.ContextSize,
		&s.MaxContextSize, &s.MessageCount, &s.TotalTokensIn, &s.TotalTokensOut,
		&s.TotalCost, &s.Metadata, &s.CreatedAt, &s.UpdatedAt,
		&pausedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("session: get failed: %w", err)
	}
	if pausedAt.Valid {
		s.PausedAt = &pausedAt.String
	}
	if completedAt.Valid {
		s.CompletedAt = &completedAt.String
	}
	return s, nil
}

type SessionFilter struct {
	AgentID string
	Status  string
	Limit   int
	Offset  int
}

func (sm *SessionManager) List(filter SessionFilter) ([]*Session, error) {
	query := `SELECT id, agent_id, title, status, context_size,
		max_context_size, message_count, total_tokens_in, total_tokens_out,
		total_cost, metadata, created_at, updated_at, paused_at, completed_at
		FROM sessions`
	var args []any
	var conditions []string
	if filter.AgentID != "" {
		conditions = append(conditions, "agent_id = ?")
		args = append(args, filter.AgentID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	query += " ORDER BY updated_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	} else {
		query += " LIMIT 50"
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}
	rows, err := sm.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("session: list failed: %w", err)
	}
	defer rows.Close()
	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		var pausedAt, completedAt sql.NullString
		if err := rows.Scan(
			&s.ID, &s.AgentID, &s.Title, &s.Status, &s.ContextSize,
			&s.MaxContextSize, &s.MessageCount, &s.TotalTokensIn, &s.TotalTokensOut,
			&s.TotalCost, &s.Metadata, &s.CreatedAt, &s.UpdatedAt,
			&pausedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("session: scan failed: %w", err)
		}
		if pausedAt.Valid {
			s.PausedAt = &pausedAt.String
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.String
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []*Session{}
	}
	return sessions, rows.Err()
}

func (sm *SessionManager) UpdateStatus(id, status string) error {
	now := Now()
	_, err := sm.db.Exec(`UPDATE sessions SET status=?, updated_at=? WHERE id=?`,
		status, now, id)
	if err != nil {
		return fmt.Errorf("session: update status failed: %w", err)
	}
	return nil
}

func (sm *SessionManager) Pause(id string) error {
	now := Now()
	_, err := sm.db.Exec(`UPDATE sessions SET status='paused', paused_at=?, updated_at=? WHERE id=?`,
		now, now, id)
	return err
}

func (sm *SessionManager) Resume(id string) error {
	_, err := sm.db.Exec(`UPDATE sessions SET status='active', paused_at=NULL, updated_at=? WHERE id=?`,
		Now(), id)
	return err
}

func (sm *SessionManager) Complete(id string) error {
	now := Now()
	_, err := sm.db.Exec(`UPDATE sessions SET status='completed', completed_at=?, updated_at=? WHERE id=?`,
		now, now, id)
	return err
}

func (sm *SessionManager) Delete(id string) error {
	res, err := sm.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("session: delete failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("session: not found")
	}
	return nil
}

func (sm *SessionManager) SetTitle(id, title string) error {
	_, err := sm.db.Exec("UPDATE sessions SET title=?, updated_at=? WHERE id=?",
		title, Now(), id)
	return err
}

func (sm *SessionManager) TrackUsage(id string, tokensIn, tokensOut int, cost float64) error {
	_, err := sm.db.Exec(`UPDATE sessions SET
		message_count = message_count + 1,
		total_tokens_in = total_tokens_in + ?,
		total_tokens_out = total_tokens_out + ?,
		total_cost = total_cost + ?,
		updated_at = ?
		WHERE id = ?`, tokensIn, tokensOut, cost, Now(), id)
	return err
}

func (sm *SessionManager) CheckExpiry(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sm.expireOld()
		case <-ctx.Done():
			return
		}
	}
}

func (sm *SessionManager) expireOld() {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	sm.db.Exec(`UPDATE sessions SET status='archived', updated_at=? WHERE status='paused' AND updated_at < ?`, Now(), cutoff)
	sm.db.Exec(`UPDATE sessions SET status='archived', updated_at=? WHERE status='completed' AND updated_at < ?`, Now(), cutoff)
}

func joinConditions(conds []string) string {
	result := conds[0]
	for i := 1; i < len(conds); i++ {
		result += " AND " + conds[i]
	}
	return result
}

func (sm *SessionManager) GetMessages(sessionID string) ([]Message, error) {
	rows, err := sm.db.Query(
		`SELECT id, session_id, role, content, model, provider, tokens_in, tokens_out, created_at
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content,
			&msg.Model, &msg.Provider, &msg.TokensIn, &msg.TokensOut, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

type rateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	limit      int
	window     time.Duration
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{windows: make(map[string]*slidingWindow)}
}

func (rl *rateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	w, exists := rl.windows[key]
	if !exists {
		w = &slidingWindow{limit: limit, window: window}
		rl.windows[key] = w
	}
	now := time.Now()
	cutoff := now.Add(-window)
	var filtered []time.Time
	for _, t := range w.timestamps {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	w.timestamps = filtered
	if len(filtered) >= limit {
		return false
	}
	w.timestamps = append(w.timestamps, now)
	return true
}

func (sm *SessionManager) RateLimitAllow(agentID string, limit int, window time.Duration) bool {
	sm.mu.Lock()
	rl, exists := sm.rateLimiters[agentID]
	if !exists {
		rl = newRateLimiter()
		sm.rateLimiters[agentID] = rl
	}
	sm.mu.Unlock()
	return rl.Allow(agentID, limit, window)
}
