/**
 * Embedding provider interface and implementations.
 * 埋め込みプロバイダーインターフェースと実装ね。
 *
 * Supports Ollama (nomic-embed-text) and OpenAI (text-embedding-3-small).
 * OllamaとOpenAIの両方をサポートしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

type OllamaEmbedder struct {
	Endpoint string
	Model    string
	client   *http.Client
}

func NewOllamaEmbedder(endpoint, model string) *OllamaEmbedder {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		Endpoint: endpoint,
		Model:    model,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OllamaEmbedder) Dimensions() int { return 768 }

func (o *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Batch into groups of 10
	var results [][]float32
	for i := 0; i < len(texts); i += 10 {
		end := i + 10
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := o.embedBatch(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		results = append(results, batch...)
	}
	return results, nil
}

func (o *OllamaEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{
		"model":  o.Model,
		"input":  texts,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("ollama embed parse: %w", err)
	}
	return result.Embeddings, nil
}

type OpenAIEmbedder struct {
	Endpoint string
	APIKey   string
	Model    string
	client   *http.Client
}

func NewOpenAIEmbedder(endpoint, apiKey, model string) *OpenAIEmbedder {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OpenAIEmbedder) Dimensions() int { return 1536 }

func (o *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var results [][]float32
	for i := 0; i < len(texts); i += 10 {
		end := i + 10
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := o.embedBatch(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		results = append(results, batch...)
	}
	return results, nil
}

func (o *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{
		"model": o.Model,
		"input": texts,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("openai embed parse: %w", err)
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

// CachedEmbedder wraps an embedder with content-addressed cache
type CachedEmbedder struct {
	Embedder
	mu     sync.RWMutex
	cache  map[string][]float32
}

func NewCachedEmbedder(inner Embedder) *CachedEmbedder {
	return &CachedEmbedder{
		Embedder: inner,
		cache:    make(map[string][]float32),
	}
}

func (c *CachedEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	c.mu.RLock()
	var results [][]float32
	var uncached []int
	var uncachedTexts []string

	for i, t := range texts {
		key := hashContent(t)
		if v, ok := c.cache[key]; ok {
			results = append(results, v)
		} else {
			results = append(results, nil) // placeholder
			uncached = append(uncached, i)
			uncachedTexts = append(uncachedTexts, t)
		}
	}
	c.mu.RUnlock()

	if len(uncachedTexts) > 0 {
		embeddings, err := c.Embedder.Embed(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		for j, idx := range uncached {
			results[idx] = embeddings[j]
			c.cache[hashContent(texts[idx])] = embeddings[j]
		}
		c.mu.Unlock()
	}

	return results, nil
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:16])
}

// EmbeddingCacheStats returns cache hit/miss info
type EmbeddingCacheStats struct {
	Size int `json:"size"`
}

func (c *CachedEmbedder) Stats() EmbeddingCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return EmbeddingCacheStats{Size: len(c.cache)}
}
