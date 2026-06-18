/**
 * Full-text search across all memory sources using SQLite FTS5.
 * SQLite FTS5を使って全メモリソースを全文検索するの。
 *
 * Sources: documents, messages, notes, user_memory
 * ソース: documents, messages, notes, user_memory
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type FTSManager struct {
	DB *sql.DB
}

type FTSResult struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	Title      string  `json:"title"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score"`
}

func NewFTSManager(db *sql.DB) *FTSManager {
	return &FTSManager{DB: db}
}

func (f *FTSManager) Initialize(ctx context.Context) error {
	if f.DB == nil {
		return nil
	}
	_, err := f.DB.ExecContext(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS fts_documents USING fts5(
			entity_type, entity_id, title, content, tokenize='unicode61'
		)
	`)
	return err
}

func (f *FTSManager) Index(ctx context.Context, entityType, entityID, title, content string) error {
	if f.DB == nil {
		return nil
	}

	// Upsert: delete existing, insert new
	_, err := f.DB.ExecContext(ctx,
		`DELETE FROM fts_documents WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID)
	if err != nil {
		return err
	}

	_, err = f.DB.ExecContext(ctx,
		`INSERT INTO fts_documents (entity_type, entity_id, title, content) VALUES (?, ?, ?, ?)`,
		entityType, entityID, title, content)
	return err
}

func (f *FTSManager) Search(ctx context.Context, query string, limit int) ([]FTSResult, error) {
	if f.DB == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}

	// Sanitize FTS5 query
	query = sanitizeFTSQuery(query)

	rows, err := f.DB.QueryContext(ctx,
		`SELECT entity_type, entity_id, title, snippet(fts_documents, 1, '<b>', '</b>', '...', 40) as snippet, rank
		 FROM fts_documents WHERE fts_documents MATCH ? ORDER BY rank LIMIT ?`,
		query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		if err := rows.Scan(&r.EntityType, &r.EntityID, &r.Title, &r.Snippet, &r.Score); err != nil {
			continue
		}
		results = append(results, r)
	}

	return results, nil
}

func (f *FTSManager) SearchAll(ctx context.Context, query string, limit int) ([]FTSResult, error) {
	results, err := f.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Also search in user_data table if available
	if f.DB != nil {
		udRows, err := f.DB.QueryContext(ctx,
			`SELECT 'user_data', id, key, value FROM user_data WHERE value LIKE ? LIMIT ?`,
			"%"+query+"%", limit)
		if err == nil {
			defer udRows.Close()
			for udRows.Next() {
				var r FTSResult
				if err := udRows.Scan(&r.EntityType, &r.EntityID, &r.Title, &r.Snippet); err == nil {
					r.Score = 0.5
					results = append(results, r)
				}
			}
		}
	}

	return results, nil
}

func (f *FTSManager) Delete(ctx context.Context, entityType, entityID string) error {
	if f.DB == nil {
		return nil
	}
	_, err := f.DB.ExecContext(ctx,
		`DELETE FROM fts_documents WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID)
	return err
}

func sanitizeFTSQuery(q string) string {
	// Remove FTS5 special characters that could cause syntax errors
	special := []string{"^", "*", "\"", "(", ")", "AND", "OR", "NOT", "NEAR"}
	for _, s := range special {
		q = strings.ReplaceAll(q, s, " ")
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// If multiple words, prefix each with + for AND matching
	words := strings.Fields(q)
	if len(words) > 1 {
		q = `"` + q + `"`
	}
	return q
}

// ── Indexing helpers ──────────────────────────────────────────────────

func (f *FTSManager) IndexDocument(ctx context.Context, docID, title, content string) error {
	return f.Index(ctx, "document", docID, title, content)
}

func (f *FTSManager) IndexMessage(ctx context.Context, msgID, sessionID, content string) error {
	return f.Index(ctx, "message", msgID, sessionID, content)
}

func (f *FTSManager) IndexNote(ctx context.Context, noteID, title, content string) error {
	return f.Index(ctx, "note", noteID, title, content)
}
