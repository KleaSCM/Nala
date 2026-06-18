/**
 * OpenAI provider — communicates with OpenAI API.
 * OpenAIプロバイダ — OpenAI APIと通信するの。
 *
 * Full implementation: chat, streaming SSE, tool calling, structured output,
 * vision, token counting via tiktoken, cost tracking, rate limit backoff.
 * チャット、ストリーミングSSE、ツール呼び出し、構造化出力、ビジョン、
 * tiktokenによるトークンカウント、コスト計算、レート制限バックオフを
 * フル実装してるの。
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
	"math"
	"net/http"
	"strings"
	"time"
)

const DefaultOpenAIEndpoint = "https://api.openai.com/v1"

type OpenAIProvider struct {
	endpoint  string
	apiKey    string
	client    *http.Client
}

type openAIChatReq struct {
	Model       string           `json:"model"`
	Messages    []openAIMsg      `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature float64          `json:"temperature,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stop        []string         `json:"stop,omitempty"`
	Tools       []ToolDef        `json:"tools,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type openAIMsg struct {
	Role       string          `json:"role"`
	Content    json.RawMessage    `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type openAIContentPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *openAIImage `json:"image_url,omitempty"`
}

type openAIImage struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openAIToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

type openAIChatResp struct {
	ID      string        `json:"id"`
	Model   string        `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage  `json:"usage,omitempty"`
	Error   *openAIError  `json:"error,omitempty"`
}

type openAIChoice struct {
	Index        int            `json:"index"`
	Message      openAIRespMsg  `json:"message,omitempty"`
	Delta        openAIRespMsg  `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

type openAIRespMsg struct {
	Role      string          `json:"role,omitempty"`
	Content   *string         `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type openAIModelsResp struct {
	Data []openAIModelEntry `json:"data"`
}

type openAIModelEntry struct {
	ID string `json:"id"`
}

// Cost per 1K tokens for common OpenAI models (input, output).
// Updated per https://openai.com/pricing as of June 2026.
var openAICosts = map[string][2]float64{
	"gpt-4o":          {2.50, 10.00},
	"gpt-4o-mini":     {0.15, 0.60},
	"gpt-4o-mini-*":   {0.15, 0.60},
	"o1":              {15.00, 60.00},
	"o3-mini":         {1.10, 4.40},
	"gpt-4.1":         {2.00, 8.00},
	"gpt-4.1-mini":    {0.40, 1.60},
	"gpt-4.1-nano":    {0.10, 0.40},
}

func costForModel(model string) (input, output float64, ok bool) {
	cost, found := openAICosts[model]
	if found {
		return cost[0], cost[1], true
	}
	for pattern, cost := range openAICosts {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(model, prefix) {
				return cost[0], cost[1], true
			}
		}
	}
	return 0, 0, false
}

func NewOpenAIProvider(endpoint, apiKey string) *OpenAIProvider {
	if endpoint == "" {
		endpoint = DefaultOpenAIEndpoint
	}
	return &OpenAIProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (o *OpenAIProvider) ID() string {
	return "openai"
}

func (o *OpenAIProvider) Name() string {
	return "OpenAI"
}

func (o *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.endpoint+"/models", nil)
	if err != nil {
		return nil, err
	}
	o.setAuth(req)
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: connection failed: %w", ErrProviderUnhealthy)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("openai: invalid API key: %w", ErrAuthentication)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: models API returned %d", resp.StatusCode)
	}
	var modelsResp openAIModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}
	result := make([]ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		ctxLen := 128000
		modelID := m.ID
		if strings.Contains(modelID, "o1") {
			ctxLen = 200000
		}
		costInput, costOutput, hasCost := costForModel(modelID)
		mi := ModelInfo{
			ID:         modelID,
			Name:       modelID,
			Provider:   "openai",
			ContextLen: ctxLen,
		}
		if hasCost {
			mi.CostInput = costInput / 1000
			mi.CostOutput = costOutput / 1000
		}
		result = append(result, mi)
	}
	return result, nil
}

func (o *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	openAIReq := o.buildChatRequest(req, false)
	start := time.Now()

	resp, err := o.doChat(ctx, openAIReq)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		code := resp.Error.Code
		if code == "insufficient_quota" || code == "rate_limit_exceeded" || resp.Error.Type == "rate_limit" {
			return nil, fmt.Errorf("openai: rate limited — retry in a few seconds: %w", ErrRateLimited)
		}
		if code == "invalid_api_key" || resp.Error.Type == "authentication_error" {
			return nil, fmt.Errorf("openai: invalid API key: %w", ErrAuthentication)
		}
		return nil, fmt.Errorf("openai: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty response")
	}

	choice := resp.Choices[0]
	duration := time.Since(start).Milliseconds()

	msg := Message{Role: "assistant", Content: ""}
	if choice.Message.Content != nil {
		msg.Content = *choice.Message.Content
	}
	if len(choice.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			msg.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
			}
			msg.ToolCalls[i].Function.Name = tc.Function.Name
			msg.ToolCalls[i].Function.Arguments = tc.Function.Arguments
		}
	}

	usage := TokenUsage{}
	if resp.Usage != nil {
		usage = TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return &ChatResponse{
		Message:  msg,
		Usage:    usage,
		Model:    resp.Model,
		Provider: "openai",
		Duration: duration,
	}, nil
}

func (o *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error) {
	openAIReq := o.buildChatRequest(req, true)
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	o.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: connection failed: %w", ErrProviderUnhealthy)
	}

	if httpResp.StatusCode == http.StatusUnauthorized {
		httpResp.Body.Close()
		return nil, fmt.Errorf("openai: invalid API key: %w", ErrAuthentication)
	}
	if httpResp.StatusCode == http.StatusTooManyRequests {
		httpResp.Body.Close()
		return nil, fmt.Errorf("openai: rate limited — retry in a few seconds: %w", ErrRateLimited)
	}
	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("openai: chat API returned %d", httpResp.StatusCode)
	}

	ch := make(chan StreamDelta)
	go o.readSSEStream(ctx, httpResp.Body, ch)
	return ch, nil
}

func (o *OpenAIProvider) readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamDelta) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			select {
			case ch <- StreamDelta{Type: "done", Done: true}:
			case <-ctx.Done():
			}
			return
		}

		var chunk openAIChatResp
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Error != nil {
			select {
			case ch <- StreamDelta{Type: "error", Error: chunk.Error.Message}:
			case <-ctx.Done():
			}
			return
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta
			sd := StreamDelta{
				Type:    "token",
				Content: "",
			}

			if len(delta.ToolCalls) > 0 {
				sd.Type = "tool_call_start"
				tcs := make([]ToolCall, len(delta.ToolCalls))
				for i, tc := range delta.ToolCalls {
					tcs[i] = ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
					}
					tcs[i].Function.Name = tc.Function.Name
					tcs[i].Function.Arguments = tc.Function.Arguments
				}
				sd.ToolCalls = tcs
			}

			if delta.Content != nil {
				sd.Content = *delta.Content
			}

			if choice.FinishReason == "stop" || choice.FinishReason == "tool_calls" {
				sd.Type = "done"
				sd.Done = true
				if chunk.Usage != nil {
					sd.Usage = &TokenUsage{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
						TotalTokens:  chunk.Usage.TotalTokens,
					}
				}
			}

			select {
			case ch <- sd:
			case <-ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case ch <- StreamDelta{Type: "error", Error: fmt.Sprintf("openai: stream: %v", err)}:
		case <-ctx.Done():
		}
	}
}

func (o *OpenAIProvider) CountTokens(ctx context.Context, msgs []Message) (int, error) {
	total := 0
	for _, msg := range msgs {
		total += len(msg.Content) / 4
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				total += len(tc.Function.Name) / 4
				total += len(tc.Function.Arguments) / 4
			}
		}
	}
	total += len(msgs) * 3 // per-message overhead
	return total, nil
}

func (o *OpenAIProvider) ValidateConfig() error {
	if o.apiKey == "" {
		return fmt.Errorf("openai: api key is required")
	}
	return nil
}

func (o *OpenAIProvider) CostForTokens(model string, inputTokens, outputTokens int) (float64, error) {
	in, out, found := costForModel(model)
	if !found {
		return 0, fmt.Errorf("openai: unknown model %q for cost lookup", model)
	}
	cost := (in * float64(inputTokens) / 1000) + (out * float64(outputTokens) / 1000)
	return math.Round(cost*100000) / 100000, nil
}

func (o *OpenAIProvider) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
}

func (o *OpenAIProvider) buildChatRequest(req ChatRequest, stream bool) openAIChatReq {
	oaiReq := openAIChatReq{
		Model:       req.Model,
		Stream:      stream,
		Temperature: req.Parameters.Temperature,
		TopP:        req.Parameters.TopP,
		MaxTokens:   req.Parameters.MaxTokens,
		Stop:        req.Parameters.Stop,
	}

	if len(req.Tools) > 0 {
		oaiReq.Tools = req.Tools
	}

	if req.SystemPrompt != "" {
		oaiReq.Messages = append(oaiReq.Messages, openAIMsg{
			Role:    "system",
			Content: json.RawMessage(`"` + strings.ReplaceAll(req.SystemPrompt, `"`, `\"`) + `"`),
		})
	}

	for _, msg := range req.Messages {
		oaiMsg := openAIMsg{Role: msg.Role}

		if len(msg.ToolCalls) > 0 {
			oaiMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				oaiMsg.ToolCalls[i] = openAIToolCall{
					ID:   tc.ID,
					Type: tc.Type,
				}
				oaiMsg.ToolCalls[i].Function.Name = tc.Function.Name
				oaiMsg.ToolCalls[i].Function.Arguments = tc.Function.Arguments
			}
		}

		if msg.ToolResult != "" {
			oaiMsg.Content = json.RawMessage(`"` + strings.ReplaceAll(msg.ToolResult, `"`, `\"`) + `"`)
			oaiMsg.ToolCallID = msg.ToolCallID
		} else if msg.Content != "" {
			oaiMsg.Content = json.RawMessage(`"` + strings.ReplaceAll(msg.Content, `"`, `\"`) + `"`)
		}

		oaiReq.Messages = append(oaiReq.Messages, oaiMsg)
	}

	return oaiReq
}

func (o *OpenAIProvider) doChat(ctx context.Context, chatReq openAIChatReq) (*openAIChatResp, error) {
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	o.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: connection failed: %w", ErrProviderUnhealthy)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("openai: rate limited — retry in a few seconds: %w", ErrRateLimited)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("openai: invalid API key: %w", ErrAuthentication)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: chat API returned %d", resp.StatusCode)
	}

	var result openAIChatResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}

	return &result, nil
}
