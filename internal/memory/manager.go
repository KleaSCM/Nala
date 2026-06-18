/**
 * Memory manager — orchestrates vector DB, embeddings, RAG, FTS, entities.
 * メモリマネージャー — ベクターDB、埋め込み、RAG、FTS、エンティティを統括するの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	DB          *sql.DB
	VectorDB    *VectorDB
	Embedder    Embedder
	RAG         *RAGPipeline
	FTS         *FTSManager
	Extractor   *EntityExtractor
	Chunker     *Chunker

	mu            sync.RWMutex
	conversations map[string]ConversationMemory
	userMemories  []UserMemory

	consolidationInterval time.Duration
	autoExtractFacts      bool
}

type ConversationMemory struct {
	SessionID string    `json:"session_id"`
	Messages  []string  `json:"messages"`
	Summary   string    `json:"summary,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserMemory struct {
	Fact       string    `json:"fact"`
	Category   string    `json:"category"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
}

func New(db *sql.DB) *Manager {
	vdb := NewVectorDB()
	chunker := NewChunker("recursive", 1000, 200)
	extractor := NewEntityExtractor()
	fts := NewFTSManager(db)

	return &Manager{
		DB:          db,
		VectorDB:    vdb,
		Chunker:     chunker,
		FTS:         fts,
		Extractor:   extractor,
		RAG:         &RAGPipeline{VectorDB: vdb},
		conversations: make(map[string]ConversationMemory),
		consolidationInterval: 60 * time.Minute,
		autoExtractFacts:      true,
	}
}

func (m *Manager) SetEmbedder(e Embedder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Embedder = e
	m.RAG = NewRAGPipeline(m.VectorDB, e, m.Chunker)
}

func (m *Manager) SetAutoExtract(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoExtractFacts = enabled
}

// ── Knowledge Base CRUD ──────────────────────────────────────────────

func (m *Manager) CreateKB(ctx context.Context, name, embeddingModel, chunkStrategy string, chunkSize, chunkOverlap int) (*KnowledgeBase, error) {
	if m.DB == nil {
		return nil, fmt.Errorf("database not available")
	}

	kb := &KnowledgeBase{
		ID:             generateID(),
		Name:           name,
		EmbeddingModel: embeddingModel,
		ChunkStrategy:  chunkStrategy,
		ChunkSize:      chunkSize,
		ChunkOverlap:   chunkOverlap,
		Metadata:       "{}",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	_, err := m.DB.ExecContext(ctx,
		`INSERT INTO knowledge_bases (id, name, description, embedding_model, chunk_strategy, chunk_size, chunk_overlap, document_count, metadata, created_at, updated_at)
		 VALUES (?, ?, '', ?, ?, ?, ?, 0, '{}', ?, ?)`,
		kb.ID, kb.Name, kb.EmbeddingModel, kb.ChunkStrategy, kb.ChunkSize, kb.ChunkOverlap, kb.CreatedAt, kb.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create kb: %w", err)
	}
	return kb, nil
}

func (m *Manager) GetKB(ctx context.Context, id string) (*KnowledgeBase, error) {
	if m.DB == nil {
		return nil, fmt.Errorf("database not available")
	}

	kb := &KnowledgeBase{}
	err := m.DB.QueryRowContext(ctx,
		`SELECT id, name, description, embedding_model, chunk_strategy, chunk_size, chunk_overlap, document_count, metadata, created_at, updated_at
		 FROM knowledge_bases WHERE id = ?`, id).Scan(
		&kb.ID, &kb.Name, &kb.Description, &kb.EmbeddingModel, &kb.ChunkStrategy,
		&kb.ChunkSize, &kb.ChunkOverlap, &kb.DocumentCount, &kb.Metadata, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get kb: %w", err)
	}
	return kb, nil
}

func (m *Manager) ListKBs(ctx context.Context) ([]*KnowledgeBase, error) {
	if m.DB == nil {
		return nil, nil
	}

	rows, err := m.DB.QueryContext(ctx,
		`SELECT id, name, description, embedding_model, chunk_strategy, chunk_size, chunk_overlap, document_count, metadata, created_at, updated_at
		 FROM knowledge_bases ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kbs []*KnowledgeBase
	for rows.Next() {
		kb := &KnowledgeBase{}
		if err := rows.Scan(&kb.ID, &kb.Name, &kb.Description, &kb.EmbeddingModel, &kb.ChunkStrategy,
			&kb.ChunkSize, &kb.ChunkOverlap, &kb.DocumentCount, &kb.Metadata, &kb.CreatedAt, &kb.UpdatedAt); err != nil {
			continue
		}
		kbs = append(kbs, kb)
	}
	return kbs, nil
}

func (m *Manager) DeleteKB(ctx context.Context, id string) error {
	if m.DB == nil {
		return fmt.Errorf("database not available")
	}
	_, err := m.DB.ExecContext(ctx, `DELETE FROM knowledge_bases WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete kb: %w", err)
	}
	m.VectorDB.Clear(ctx, "kb_"+id)
	return nil
}

// ── Document management ──────────────────────────────────────────────

func (m *Manager) IngestDocument(ctx context.Context, filePath, kbID string) (*Document, error) {
	processor := NewDocumentProcessor()
	processed, err := processor.Process(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	if processed.IsBinary {
		return m.storeDocument(ctx, kbID, processed.Filename, "", processed.ContentHash, processed.MimeType, "[]", "{}"), nil
	}

	chunks, err := m.Chunker.Chunk(ctx, processed.Content)
	if err != nil {
		return nil, fmt.Errorf("chunk: %w", err)
	}

	chunksJSON, _ := jsonMarshal(chunks)
	doc := m.storeDocument(ctx, kbID, processed.Filename, processed.Content, processed.ContentHash, processed.MimeType, string(chunksJSON), "{}")

	// Embed and store chunks in vector DB
	if m.Embedder != nil && len(chunks) > 0 {
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Content
		}
		vectors, err := m.Embedder.Embed(ctx, texts)
		if err == nil {
			collection := "kb_" + kbID
			for i, v := range vectors {
				if i < len(vectors) {
					chunkID := fmt.Sprintf("%s_chunk_%d", doc.ID, i)
					m.VectorDB.Insert(ctx, collection, chunkID, v, chunks[i].Content)
				}
			}
		}
	}

	// Index in FTS
	if m.FTS != nil {
		m.FTS.IndexDocument(ctx, doc.ID, processed.Filename, processed.Content)
		chunkTexts := make([]string, len(chunks))
		for i, c := range chunks {
			chunkTexts[i] = c.Content
		}
		m.FTS.Index(ctx, "document_content", doc.ID, processed.Filename, strings.Join(chunkTexts, "\n\n"))
	}

	return doc, nil
}

func (m *Manager) storeDocument(ctx context.Context, kbID, filename, content, contentHash, mimeType, chunks, metadata string) *Document {
	doc := &Document{
		ID:          generateID(),
		KBID:        kbID,
		Filename:    filename,
		Content:     content,
		ContentHash: contentHash,
		MimeType:    mimeType,
		Chunks:      chunks,
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if m.DB != nil {
		m.DB.ExecContext(ctx,
			`INSERT INTO documents (id, kb_id, filename, content, content_hash, mime_type, chunks, metadata, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.ID, doc.KBID, doc.Filename, doc.Content, doc.ContentHash, doc.MimeType, doc.Chunks, doc.Metadata, doc.CreatedAt)
	}

	return doc
}

// ── Conversation memory ──────────────────────────────────────────────

func (m *Manager) AddMessages(ctx context.Context, sessionID string, messages []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.conversations[sessionID]
	if !ok {
		entry = ConversationMemory{
			SessionID: sessionID,
			UpdatedAt: time.Now(),
		}
	}
	entry.Messages = append(entry.Messages, messages...)
	entry.UpdatedAt = time.Now()
	m.conversations[sessionID] = entry

	// Auto-extract entities
	for _, msg := range messages {
		m.Extractor.Store(msg)
	}

	// Auto-extract facts
	if m.autoExtractFacts {
		for _, msg := range messages {
			facts := extractFacts(msg)
			for _, fact := range facts {
				m.addUserMemoryLocked(fact, "fact", 0.7, "auto_extracted")
			}
		}
	}
}

func (m *Manager) GetConversation(ctx context.Context, sessionID string) (ConversationMemory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.conversations[sessionID]
	return entry, ok
}

func (m *Manager) SummarizeConversation(ctx context.Context, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.conversations[sessionID]
	if !ok || len(entry.Messages) == 0 {
		return "", nil
	}

	// Simple extractive summary: take first message context + last message
	var b strings.Builder
	if len(entry.Messages) > 0 {
		b.WriteString("Conversation started with: ")
		b.WriteString(truncateText(entry.Messages[0], 200))
	}
	if len(entry.Messages) > 1 {
		b.WriteString("\nLatest message: ")
		b.WriteString(truncateText(entry.Messages[len(entry.Messages)-1], 200))
	}
	b.WriteString(fmt.Sprintf("\nTotal messages: %d", len(entry.Messages)))
	summary := b.String()

	entry.Summary = summary
	m.conversations[sessionID] = entry
	return summary, nil
}

// ── User memory ──────────────────────────────────────────────────────

func (m *Manager) StoreUserMemory(ctx context.Context, fact, category string, importance float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addUserMemoryLocked(fact, category, importance, "explicit")
	return nil
}

func (m *Manager) addUserMemoryLocked(fact, category string, confidence float64, source string) {
	// Dedup by cosine similarity
	for _, existing := range m.userMemories {
		if strings.EqualFold(existing.Fact, fact) {
			existing.Confidence = max(existing.Confidence, confidence)
			return
		}
	}

	m.userMemories = append(m.userMemories, UserMemory{
		Fact:       fact,
		Category:   category,
		Confidence: confidence,
		Source:     source,
		CreatedAt:  time.Now(),
	})
}

func (m *Manager) RecallUserMemory(ctx context.Context, query string, topK int) ([]UserMemory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if topK <= 0 || topK > 50 {
		topK = 10
	}

	queryLower := strings.ToLower(query)
	type scored struct {
		mem   UserMemory
		score float64
	}
	var scoredMems []scored

	for _, mem := range m.userMemories {
		score := 0.0
		if strings.Contains(strings.ToLower(mem.Fact), queryLower) {
			score = 0.9
		} else {
			// Fuzzy word match
			for _, word := range strings.Fields(queryLower) {
				if strings.Contains(strings.ToLower(mem.Fact), word) {
					score += 0.3
				}
			}
		}
		score = score * mem.Confidence
		if score > 0 {
			scoredMems = append(scoredMems, scored{mem, score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scoredMems); i++ {
		for j := i + 1; j < len(scoredMems); j++ {
			if scoredMems[j].score > scoredMems[i].score {
				scoredMems[i], scoredMems[j] = scoredMems[j], scoredMems[i]
			}
		}
	}

	if len(scoredMems) > topK {
		scoredMems = scoredMems[:topK]
	}

	result := make([]UserMemory, len(scoredMems))
	for i, s := range scoredMems {
		result[i] = s.mem
	}
	return result, nil
}

func (m *Manager) GetAllUserMemories() []UserMemory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]UserMemory, len(m.userMemories))
	copy(result, m.userMemories)
	return result
}

// ── Memory consolidation ─────────────────────────────────────────────

func (m *Manager) StartConsolidationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.consolidationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.consolidate(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) consolidate(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sessionID, entry := range m.conversations {
		if len(entry.Messages) > 50 && now.Sub(entry.UpdatedAt) > time.Hour {
			summary, _ := m.SummarizeConversation(ctx, sessionID)
			if summary != "" {
				entry.Summary = summary
				// Keep last 20 messages, summarize rest
				if len(entry.Messages) > 20 {
					entry.Messages = entry.Messages[len(entry.Messages)-20:]
				}
				m.conversations[sessionID] = entry
			}
		}
	}
}

// ── Multi-hop retrieval ──────────────────────────────────────────────

func (m *Manager) MultiHopQuery(ctx context.Context, query string, maxHops int) (*RAGResult, error) {
	if maxHops <= 0 || maxHops > 5 {
		maxHops = 5
	}

	// Hop 1: Extract entities from query
	entities := m.Extractor.Extract(query)

	// Hop 2: Search vector DB with query
	var allSources []RAGSource
	seen := make(map[string]bool)

	for hop := 0; hop < maxHops; hop++ {
		var searchQuery string
		switch hop {
		case 0:
			searchQuery = query
		case 1:
			// Search by extracted entity values
			var parts []string
			for _, e := range entities {
				parts = append(parts, e.Value)
			}
			searchQuery = strings.Join(parts, " ")
		default:
			// Search user memories and conversations
			memories, _ := m.RecallUserMemory(ctx, query, 5)
			for _, mem := range memories {
				if !seen[mem.Fact] {
					allSources = append(allSources, RAGSource{
						ID:       mem.Fact,
						Filename: "user_memory",
						Score:    mem.Confidence,
						Snippet:  mem.Fact,
					})
					seen[mem.Fact] = true
				}
			}
			continue
		}

		if searchQuery == "" {
			continue
		}

		var result *RAGResult
		if m.Embedder != nil {
			r, err := m.RAG.Query(ctx, searchQuery, "", 5, 0.5)
			if err == nil {
				result = r
			}
		}

		if result != nil {
			for _, src := range result.Sources {
				if !seen[src.ID] {
					allSources = append(allSources, src)
					seen[src.ID] = true
				}
			}
		}
	}

	var b strings.Builder
	if len(allSources) > 0 {
		b.WriteString("## Relevant Information\n\n")
		for _, src := range allSources {
			b.WriteString(fmt.Sprintf("- **%s** (score: %.2f): %s\n", src.Filename, src.Score, src.Snippet))
		}
	} else {
		b.WriteString("No relevant information found in memory or knowledge base. I don't know the answer to that yet.\n")
		b.WriteString("I'll remember this question and try to learn more over time.\n")
	}

	return &RAGResult{
		Sources:   allSources,
		Context:   b.String(),
		TotalHops: maxHops,
	}, nil
}

// ── Query interface ─────────────────────────────────────────────────

type MemoryQuery struct {
	Text         string   `json:"text"`
	EntityTypes  []string `json:"entity_types,omitempty"`
	TimeRange    string   `json:"time_range,omitempty"`
	Sources      []string `json:"sources,omitempty"`
	MaxHops      int      `json:"max_hops"`
	MinConfidence float64 `json:"min_confidence"`
}

type MemoryResponse struct {
	Answer      string       `json:"answer"`
	Confidence  float64      `json:"confidence"`
	Hops        []HopLog     `json:"hops,omitempty"`
	Sources     []RAGSource  `json:"sources,omitempty"`
	Entities    []Entity     `json:"entities,omitempty"`
	TimeToLiveMs int64       `json:"time_to_live_ms"`
}

type HopLog struct {
	HopNumber   int    `json:"hop_number"`
	Query       string `json:"query"`
	ResultCount int    `json:"result_count"`
	Confidence  float64 `json:"confidence"`
	DurationMs  int64  `json:"duration_ms"`
}

func (m *Manager) QueryMemory(ctx context.Context, q MemoryQuery) (*MemoryResponse, error) {
	if q.MaxHops <= 0 {
		q.MaxHops = 3
	}
	if q.MinConfidence <= 0 {
		q.MinConfidence = 0.3
	}

	start := time.Now()
	resp := &MemoryResponse{
		Confidence: 0,
	}

	// Auto-detect entity types from query
	entities := m.Extractor.Extract(q.Text)
	resp.Entities = entities

	// Search FTS
	ftsResults, _ := m.FTS.Search(ctx, q.Text, 10)
	for _, f := range ftsResults {
		resp.Sources = append(resp.Sources, RAGSource{
			ID:       f.EntityID,
			Filename: f.EntityType + ": " + f.Title,
			Score:    f.Score,
			Snippet:  f.Snippet,
		})
		if f.Score > resp.Confidence {
			resp.Confidence = f.Score
		}
	}

	// Multi-hop RAG
	ragResult, err := m.MultiHopQuery(ctx, q.Text, q.MaxHops)
	if err == nil && ragResult != nil {
		seen := make(map[string]bool)
		for _, s := range ragResult.Sources {
			if !seen[s.ID] {
				resp.Sources = append(resp.Sources, s)
				seen[s.ID] = true
				if s.Score > resp.Confidence {
					resp.Confidence = s.Score
				}
			}
		}
		resp.Answer = ragResult.Context
	} else {
		resp.Answer = "I don't have enough information to answer that yet."
	}

	// Fallback: if no sources, return recent user memories
	if len(resp.Sources) == 0 {
		memories := m.GetAllUserMemories()
		if len(memories) > 0 {
			var b strings.Builder
			b.WriteString("Here's what I know about you:\n\n")
			for _, mem := range memories {
				b.WriteString(fmt.Sprintf("- [%s] %s (confidence: %.2f)\n", mem.Category, mem.Fact, mem.Confidence))
			}
			resp.Answer = b.String()
			resp.Confidence = 0.5
		} else {
			resp.Answer = "I don't have enough information to answer that yet.\n"
			resp.Answer += "I'll remember this question and try to learn more over time.\n"
		}
	}

	resp.TimeToLiveMs = time.Since(start).Milliseconds()
	return resp, nil
}

// ── Helpers ─────────────────────────────────────────────────────────

func generateID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func jsonMarshal(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(buf.String())), nil
}

func extractFacts(text string) []string {
	// Simple rule-based fact extraction
	var facts []string

	patterns := []struct {
		re   *regexp.Regexp
		tmpl string
	}{
		{regexp.MustCompile(`(?i)(?:my name is|i'm|i am) (\w+)`), "User's name is $1"},
		{regexp.MustCompile(`(?i)i (?:work|am working) (?:as|on|at) (.+?)(?:\.|,|$)`), "User works as $1"},
		{regexp.MustCompile(`(?i)i (?:like|love|enjoy) (.+?)(?:\.|,|$)`), "User likes $1"},
		{regexp.MustCompile(`(?i)i (?:use|use |use )(\w+)`), "User uses $1"},
		{regexp.MustCompile(`(?i)(?:my|the) project (?:is|name is|called) (.+?)(?:\.|,|$)`), "User's project is $1"},
		{regexp.MustCompile(`(?i)i live in (.+?)(?:\.|,|$)`), "User lives in $1"},
	}

	for _, p := range patterns {
		matches := p.re.FindStringSubmatch(text)
		if len(matches) > 1 {
			fact := strings.Replace(p.tmpl, "$1", matches[1], 1)
			facts = append(facts, fact)
		}
	}

	return facts
}


