/**
 * Ollama provider — communicates with local Ollama server.
 * Ollamaプロバイダ — ローカルのOllamaサーバーと通信するの。
 *
 * Implements Provider interface via HTTP calls to the Ollama REST API.
 * Uses server-sent events for streaming and handles model pull progress.
 * Ollama REST APIへのHTTP呼び出しでProviderインターフェースを実装してるの。
 * ストリーミングにはサーバー送信イベントを使って、モデルプル進捗も処理するわ。
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

type OllamaProvider struct {
	endpoint string
	client   *http.Client
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Model     string       `json:"model"`
	CreatedAt string       `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool         `json:"done"`
	EvalCount int          `json:"eval_count"`
	PromptEvalCount int   `json:"prompt_eval_count"`
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
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
		return nil, fmt.Errorf("ollama: connection refused — is Ollama running?: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: tags API returned %d", resp.StatusCode)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("ollama: decode tags: %w", err)
	}

	models := make([]ModelInfo, 0, len(tags.Models))
	for _, m := range tags.Models {
		models = append(models, ModelInfo{
			ID:         m.Name,
			Name:       m.Name,
			Provider:   "ollama",
			ContextLen: 128000, // typical default, varies by model
		})
	}
	return models, nil
}

func (o *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	ollamaReq := o.buildRequest(req, false)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: chat request failed — is Ollama running?: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ollama: model %q not found — pull it from model manager: %w", req.Model, ErrModelNotFound)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: chat API returned %d", httpResp.StatusCode)
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	startTime := time.Now()
	_ = startTime // will use for duration tracking

	return &ChatResponse{
		Message: Message{
			Role:    "assistant",
			Content: ollamaResp.Message.Content,
		},
		Usage: TokenUsage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
			TotalTokens:  ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
		Model:    ollamaResp.Model,
		Provider: "ollama",
	}, nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	ollamaReq := o.buildRequest(req, true)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: stream connection failed — is Ollama running?: %w", err)
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

		var ollamaResp ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			ch <- StreamDelta{Error: fmt.Sprintf("ollama: parse error: %v", err)}
			return
		}

		delta := StreamDelta{
			Content: ollamaResp.Message.Content,
			Done:    ollamaResp.Done,
		}

		if ollamaResp.Done {
			delta.Usage = &TokenUsage{
				InputTokens:  ollamaResp.PromptEvalCount,
				OutputTokens: ollamaResp.EvalCount,
				TotalTokens:  ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
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
		case ch <- StreamDelta{Error: fmt.Sprintf("ollama: stream read error: %v", err)}:
		case <-ctx.Done():
		}
	}
}

func (o *OllamaProvider) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	// Ollama doesn't expose a standalone token count endpoint.
	// Fallback: estimate via character count / 4 (rough approximation).
	// Ollamaは単独のトークンカウントAPIがないの。文字数/4で大まかに近似するわ。
	chars := 0
	for _, msg := range msgs {
		chars += len(msg.Content)
	}
	return chars / 4, nil
}

func (o *OllamaProvider) ValidateConfig() error {
	if o.endpoint == "" {
		return fmt.Errorf("ollama: endpoint is required")
	}
	return nil
}

func (o *OllamaProvider) buildRequest(req ChatRequest, stream bool) ollamaChatRequest {
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	options := make(map[string]any)
	if req.Parameters.Temperature != 0 {
		options["temperature"] = req.Parameters.Temperature
	}
	if req.Parameters.TopP != 0 {
		options["top_p"] = req.Parameters.TopP
	}
	if req.Parameters.MaxTokens != 0 {
		options["num_predict"] = req.Parameters.MaxTokens
	}

	return ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
		Options:  options,
	}
}
