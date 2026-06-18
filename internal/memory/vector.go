/**
 * Vector database — in-memory with cosine similarity search.
 * ベクターデータベース — インメモリでコサイン類似度検索するの。
 *
 * Collections: documents, conversations, user_memory
 * Collection: documents, conversations, user_memory ね。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

type VectorDB struct {
	mu          sync.RWMutex
	collections map[string][]VectorEntry
}

type VectorEntry struct {
	ID       string
	Vector   []float32
	Metadata string
}

type VectorResult struct {
	ID       string  `json:"id"`
	Score    float64 `json:"score"`
	Metadata string  `json:"metadata"`
}

func NewVectorDB() *VectorDB {
	return &VectorDB{
		collections: make(map[string][]VectorEntry),
	}
}

func (v *VectorDB) Insert(ctx context.Context, collection, id string, vector []float32, metadata string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Check for existing and replace
	for i, e := range v.collections[collection] {
		if e.ID == id {
			v.collections[collection][i] = VectorEntry{ID: id, Vector: vector, Metadata: metadata}
			return nil
		}
	}
	v.collections[collection] = append(v.collections[collection], VectorEntry{ID: id, Vector: vector, Metadata: metadata})
	return nil
}

func (v *VectorDB) Search(ctx context.Context, collection string, query []float32, topK int) ([]VectorResult, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entries, ok := v.collections[collection]
	if !ok {
		return nil, nil
	}

	type scored struct {
		entry VectorEntry
		score float64
	}
	var results []scored

	for _, e := range entries {
		score := cosineSimilarity(query, e.Vector)
		results = append(results, scored{e, score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	out := make([]VectorResult, len(results))
	for i, r := range results {
		out[i] = VectorResult{ID: r.entry.ID, Score: r.score, Metadata: r.entry.Metadata}
	}
	return out, nil
}

func (v *VectorDB) SearchWithMinScore(ctx context.Context, collection string, query []float32, topK int, minScore float64) ([]VectorResult, error) {
	results, err := v.Search(ctx, collection, query, topK)
	if err != nil {
		return nil, err
	}
	var filtered []VectorResult
	for _, r := range results {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func (v *VectorDB) Delete(ctx context.Context, collection, id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	entries := v.collections[collection]
	for i, e := range entries {
		if e.ID == id {
			v.collections[collection] = append(entries[:i], entries[i+1:]...)
			return nil
		}
	}
	return nil
}

func (v *VectorDB) CollectionStats(ctx context.Context, collection string) (int, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.collections[collection]), nil
}

func (v *VectorDB) Clear(ctx context.Context, collection string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.collections, collection)
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
