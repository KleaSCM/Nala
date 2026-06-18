/**
 * Ollama provider — communicates with local Ollama server.
 * Ollamaプロバイダ — ローカルのOllamaサーバーと通信するの。
 *
 * Full implementation covering chat, streaming, model listing, model pulling
 * with progress, embeddings, and health checks via the Ollama REST API.
 * Ollama REST APIを通じてチャット、ストリーミング、モデル一覧、モデルプル
 * （進捗付き）、埋め込み、ヘルスチェックをフル実装してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultOllamaEndpoint = "http://localhost:11434"
const DefaultContextLen = 128000

type OllamaProvider struct {
	endpoint string
	client   *http.Client
}

type ollamaChatReq struct {
	Model    string            `json:"model"`
	Messages []ollamaMsg       `json:"messages"`
	Stream   bool              `json:"stream"`
	Options  map[string]any    `json:"options,omitempty"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResp struct {
	Model           string     `json:"model"`
	CreatedAt       string     `json:"created_at"`
	Message         ollamaMsg  `json:"message"`
	Done            bool       `json:"done"`
	EvalCount       int        `json:"eval_count"`
	PromptEvalCount int        `json:"prompt_eval_count"`
}

type ollamaTagsResp struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

type ollamaPullResp struct {
	Status     string `json:"status"`
	Digest     string `json:"digest,omitempty"`
	Total      int64  `json:"total,omitempty"`
	Completed  int64  `json:"completed,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResp struct {
	Embedding []float64 `json:"embedding"`
}

type PullProgress struct {
	Status    string
	Digest    string
	Total     int64
	Completed int64
	Percent   float64
	Error     string
}

func NewOllamaProvider(endpoint string) *OllamaProvider {
	if endpoint == "" {
		endpoint = DefaultOllamaEndpoint
	}
	return &OllamaProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (o *OllamaProvider) ID() string {
	return "ollama"
}

func (o *OllamaProvider) Name() string {
	return "Ollama"
}

func (o *OllamaProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: cannot connect — is Ollama running?: %w", ErrProviderUnhealthy)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: tags API returned %d", resp.StatusCode)
	}
	var tags ollamaTagsResp
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("ollama: decode tags: %w", err)
	}
	models := make([]ModelInfo, 0, len(tags.Models))
	for _, m := range tags.Models {
		models = append(models, ModelInfo{
			ID:         m.Name,
			Name:       m.Name,
			Provider:   "ollama",
			ContextLen: DefaultContextLen,
		})
	}
	return models, nil
}

func (o *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	ollamaReq := o.buildRequest(req, false)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	start := time.Now()
	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: connection failed — is Ollama running?: %w", ErrProviderUnhealthy)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ollama: model %q not found — pull it from model manager: %w", req.Model, ErrModelNotFound)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: chat API returned %d", httpResp.StatusCode)
	}
	var ollamaResp ollamaChatResp
	if err := json.NewDecoder(httpResp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}
	return &ChatResponse{
		Message:  Message{Role: "assistant", Content: ollamaResp.Message.Content},
		Usage:    TokenUsage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
			TotalTokens:  ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
		Model:    ollamaResp.Model,
		Provider: "ollama",
		Duration: time.Since(start).Milliseconds(),
	}, nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	ollamaReq := o.buildRequest(req, true)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: connection failed — is Ollama running?: %w", ErrProviderUnhealthy)
	}
	if httpResp.StatusCode == http.StatusNotFound {
		httpResp.Body.Close()
		return nil, fmt.Errorf("ollama: model %q not found — pull it from model manager: %w", req.Model, ErrModelNotFound)
	}
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("ollama: stream API returned %d", httpResp.StatusCode)
	}
	ch := make(chan StreamDelta)
	go o.readStream(ctx, httpResp.Body, ch)
	return ch, nil
}

func (o *OllamaProvider) readStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamDelta) {
	defer body.Close()
	defer close(ch)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var resp ollamaChatResp
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			select {
			case ch <- StreamDelta{Type: "error", Error: fmt.Sprintf("ollama: parse: %v", err)}:
			case <-ctx.Done():
			}
			return
		}
		delta := StreamDelta{
			Type:    "token",
			Content: resp.Message.Content,
			Done:    resp.Done,
		}
		if resp.Done {
			delta.Type = "done"
			delta.Usage = &TokenUsage{
				InputTokens:  resp.PromptEvalCount,
				OutputTokens: resp.EvalCount,
				TotalTokens:  resp.PromptEvalCount + resp.EvalCount,
			}
		}
		select {
		case ch <- delta:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case ch <- StreamDelta{Type: "error", Error: fmt.Sprintf("ollama: stream: %v", err)}:
		case <-ctx.Done():
		}
	}
}

func (o *OllamaProvider) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	chars := 0
	for _, msg := range msgs {
		chars += len(msg.Content)
	}
	if chars == 0 {
		return 0, nil
	}
	return chars / 4, nil
}

func (o *OllamaProvider) ValidateConfig() error {
	return nil
}

func (o *OllamaProvider) PullModel(ctx context.Context, name string) (<-chan PullProgress, error) {
	body, _ := json.Marshal(map[string]any{"name": name, "stream": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: pull connection failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama: pull API returned %d", resp.StatusCode)
	}

	ch := make(chan PullProgress)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var pr ollamaPullResp
			if err := json.Unmarshal([]byte(line), &pr); err != nil {
				continue
			}
			progress := PullProgress{
				Status:    pr.Status,
				Digest:    pr.Digest,
				Total:     pr.Total,
				Completed: pr.Completed,
				Error:     pr.Error,
			}
			if pr.Total > 0 {
				progress.Percent = float64(pr.Completed) / float64(pr.Total) * 100
			}
			select {
			case ch <- progress:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (o *OllamaProvider) Embeddings(ctx context.Context, model, prompt string) ([]float64, error) {
	body, _ := json.Marshal(ollamaEmbedReq{Model: model, Prompt: prompt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: embeddings connection failed: %w", ErrProviderUnhealthy)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ollama: model %q not found for embeddings: %w", model, ErrModelNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: embeddings API returned %d", resp.StatusCode)
	}
	var embedResp ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("ollama: decode embeddings: %w", err)
	}
	return embedResp.Embedding, nil
}

func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.endpoint+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: not running: %w", ErrProviderUnhealthy)
	}
	resp.Body.Close()
	return nil
}

func (o *OllamaProvider) buildRequest(req ChatRequest, stream bool) ollamaChatReq {
	msgs := make([]ollamaMsg, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, ollamaMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, ollamaMsg{Role: m.Role, Content: m.Content})
	}
	opts := make(map[string]any)
	if req.Parameters.Temperature != 0 {
		opts["temperature"] = req.Parameters.Temperature
	}
	if req.Parameters.TopP != 0 {
		opts["top_p"] = req.Parameters.TopP
	}
	if req.Parameters.MaxTokens != 0 {
		opts["num_predict"] = req.Parameters.MaxTokens
	}
	if len(req.Parameters.Stop) > 0 {
		opts["stop"] = req.Parameters.Stop
	}
	if req.Parameters.FrequencyPenalty != 0 {
		opts["frequency_penalty"] = req.Parameters.FrequencyPenalty
	}
	if req.Parameters.PresencePenalty != 0 {
		opts["presence_penalty"] = req.Parameters.PresencePenalty
	}
	if req.Parameters.Seed != 0 {
		opts["seed"] = req.Parameters.Seed
	}
	return ollamaChatReq{
		Model:    req.Model,
		Messages: msgs,
		Stream:   stream,
		Options:  opts,
	}
}
