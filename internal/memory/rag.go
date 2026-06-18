/**
 * RAG pipeline — embed query, vector search, format context.
 * RAGパイプライン — クエリを埋め込んで、ベクター検索して、コンテキストを整形するの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"context"
	"fmt"
	"strings"
)

type RAGPipeline struct {
	VectorDB  *VectorDB
	Embedder  Embedder
	Chunker   *Chunker
}

func NewRAGPipeline(vdb *VectorDB, embedder Embedder, chunker *Chunker) *RAGPipeline {
	return &RAGPipeline{
		VectorDB: vdb,
		Embedder: embedder,
		Chunker:  chunker,
	}
}

type RAGResult struct {
	Sources     []RAGSource `json:"sources"`
	Context     string      `json:"context"`
	TotalHops   int         `json:"total_hops"`
	TimeToLiveMs int64      `json:"time_to_live_ms"`
}

type RAGSource struct {
	ID       string  `json:"id"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
	Snippet  string  `json:"snippet"`
}

func (r *RAGPipeline) Query(ctx context.Context, query string, kbID string, topK int, minScore float64) (*RAGResult, error) {
	if r.Embedder == nil {
		return &RAGResult{}, nil
	}

	vectors, err := r.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("rag embed: %w", err)
	}
	if len(vectors) == 0 {
		return &RAGResult{}, nil
	}

	collection := "documents"
	if kbID != "" {
		collection = "kb_" + kbID
	}

	results, err := r.VectorDB.SearchWithMinScore(ctx, collection, vectors[0], topK, minScore)
	if err != nil {
		return nil, fmt.Errorf("rag search: %w", err)
	}

	var sources []RAGSource
	var contextParts []string

	for _, res := range results {
		sources = append(sources, RAGSource{
			ID:       res.ID,
			Filename: extractFilename(res.Metadata),
			Score:    res.Score,
			Snippet:  truncateText(res.Metadata, 200),
		})
		contextParts = append(contextParts, fmt.Sprintf("Source: %s (score: %.2f)\n%s", extractFilename(res.Metadata), res.Score, res.Metadata))
	}

	contextStr := strings.Join(contextParts, "\n\n---\n\n")

	return &RAGResult{
		Sources: sources,
		Context: contextStr,
	}, nil
}

func (r *RAGPipeline) FormatContext(result *RAGResult) string {
	if result == nil || len(result.Sources) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Relevant context from knowledge base:\n\n")
	b.WriteString(result.Context)
	b.WriteString("\n\nUse this context to answer the user's question.")
	return b.String()
}

func extractFilename(metadata string) string {
	if metadata == "" {
		return "unknown"
	}
	lines := strings.SplitN(metadata, "\n", 2)
	return strings.TrimSpace(lines[0])
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
