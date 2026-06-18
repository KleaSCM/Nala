/**
 * Model fallback chains — try providers in order, fall through on failure.
 * モデルフォールバックチェーン — プロバイダを順番に試して、失敗したら次に進むの。
 *
 * Triggers: 5xx errors, timeouts, rate limits, connection failures, refusal detection.
 * Each fallback is logged. If all fail, the last error is returned.
 * トリガー: 5xxエラー、タイムアウト、レート制限、接続失敗、拒否検出。
 * 各フォールバックは記録されるわ。すべて失敗したら最後のエラーを返すの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type FallbackStep struct {
	Provider string
	Model    string
}

type FallbackChain struct {
	Steps      []FallbackStep
	MaxRetries int
	timeout    time.Duration
}

func NewFallbackChain(steps []FallbackStep) *FallbackChain {
	return &FallbackChain{
		Steps:      steps,
		MaxRetries: 1,
		timeout:    120 * time.Second,
	}
}

func (fc *FallbackChain) SetTimeout(d time.Duration) {
	fc.timeout = d
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrProviderUnhealthy) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrModelNotFound) ||
		errors.Is(err, ErrRefusalDetected) ||
		errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "5xx") ||
		strings.Contains(msg, "5") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "refused")
}

type FallbackResult struct {
	Response  *ChatResponse
	Error     error
	Provider  string
	Model     string
	Attempted int
	Total     int
}

func (r *Registry) ChatWithFallback(ctx context.Context, chain *FallbackChain, req ChatRequest) *FallbackResult {
	ctx, cancel := context.WithTimeout(ctx, chain.timeout)
	defer cancel()

	lastErr := error(nil)

	for i, step := range chain.Steps {
		provider, err := r.Get(step.Provider)
		if err != nil {
			lastErr = fmt.Errorf("fallback: provider %q unavailable: %w", step.Provider, err)
			continue
		}

		req.Model = step.Model

		resp, chatErr := provider.Chat(ctx, req)
		if chatErr == nil {
			return &FallbackResult{
				Response:  resp,
				Provider:  step.Provider,
				Model:     step.Model,
				Attempted: i + 1,
				Total:     len(chain.Steps),
			}
		}

		lastErr = chatErr

		if !isRetryable(chatErr) {
			return &FallbackResult{
				Error:     chatErr,
				Provider:  step.Provider,
				Model:     step.Model,
				Attempted: i + 1,
				Total:     len(chain.Steps),
			}
		}

		if i < len(chain.Steps)-1 && chain.MaxRetries > 0 {
			for retry := 0; retry < chain.MaxRetries; retry++ {
				resp, retryErr := provider.Chat(ctx, req)
				if retryErr == nil {
					return &FallbackResult{
						Response:  resp,
						Provider:  step.Provider,
						Model:     step.Model,
						Attempted: i + 1,
						Total:     len(chain.Steps),
					}
				}
				lastErr = retryErr
			}
		}
	}

	return &FallbackResult{
		Error:     fmt.Errorf("fallback: all %d steps exhausted: %w", len(chain.Steps), lastErr),
		Attempted: len(chain.Steps),
		Total:     len(chain.Steps),
	}
}

func (r *Registry) ChatStreamWithFallback(ctx context.Context, chain *FallbackChain, req ChatRequest) (<-chan StreamDelta, error) {
	ctx, cancel := context.WithTimeout(ctx, chain.timeout)
	defer cancel()

	for i, step := range chain.Steps {
		provider, err := r.Get(step.Provider)
		if err != nil {
			continue
		}

		req.Model = step.Model
		ch, streamErr := provider.ChatStream(ctx, req)
		if streamErr == nil {
			return ch, nil
		}

		if !isRetryable(streamErr) {
			break
		}

		if i == len(chain.Steps)-1 {
			return nil, fmt.Errorf("fallback: all %d steps exhausted: %w", len(chain.Steps), streamErr)
		}
	}

	return nil, fmt.Errorf("fallback: all %d steps exhausted", len(chain.Steps))
}
