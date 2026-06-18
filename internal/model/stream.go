/**
 * Streaming event manager — bridges provider streaming to the UI.
 * ストリーミングイベントマネージャ — プロバイダのストリームをUIに橋渡しするの。
 *
 * Wraps provider ChatStream channels, emits typed events via Wails EventsEmit,
 * and provides a clean consumer interface for the Svelte frontend.
 * プロバイダのChatStreamチャンネルをラップして、Wails EventsEmitで型付きイベントを
 * 発行するの。Svelteフロントエンド向けのクリーンなコンシューマーインターフェースも
 * 提供するわ。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"sync"
)

type StreamEventType string

const (
	EventToken        StreamEventType = "token"
	EventToolCall     StreamEventType = "tool_call"
	EventToolResult   StreamEventType = "tool_result"
	EventDone         StreamEventType = "done"
	EventError        StreamEventType = "error"
	EventModelSwitch  StreamEventType = "model_switch"
)

type StreamEvent struct {
	Type      StreamEventType `json:"type"`
	SessionID string          `json:"session_id"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
	Usage     *TokenUsage     `json:"usage,omitempty"`
	Model     string          `json:"model,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type EventEmitter func(event StreamEvent)

type StreamManager struct {
	mu       sync.RWMutex
	emitters []EventEmitter
	streams  map[string]context.CancelFunc
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]context.CancelFunc),
	}
}

func (sm *StreamManager) AddEmitter(emitter EventEmitter) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.emitters = append(sm.emitters, emitter)
}

func (sm *StreamManager) emit(event StreamEvent) {
	sm.mu.RLock()
	emitters := make([]EventEmitter, len(sm.emitters))
	copy(emitters, sm.emitters)
	sm.mu.RUnlock()

	for _, e := range emitters {
		e(event)
	}
}

func (sm *StreamManager) ConsumeStream(ctx context.Context, sessionID string, deltas <-chan StreamDelta) {
	ctx, cancel := context.WithCancel(ctx)

	sm.mu.Lock()
	if oldCancel, exists := sm.streams[sessionID]; exists {
		oldCancel()
	}
	sm.streams[sessionID] = cancel
	sm.mu.Unlock()

	go func() {
		defer func() {
			sm.mu.Lock()
			delete(sm.streams, sessionID)
			sm.mu.Unlock()
		}()

		for {
			select {
			case delta, ok := <-deltas:
				if !ok {
					sm.emit(StreamEvent{
						Type:      EventDone,
						SessionID: sessionID,
					})
					return
				}

				if delta.Error != "" {
					sm.emit(StreamEvent{
						Type:      EventError,
						SessionID: sessionID,
						Error:     delta.Error,
					})
					return
				}

				if delta.Done {
					sm.emit(StreamEvent{
						Type:      EventDone,
						SessionID: sessionID,
						Usage:     delta.Usage,
					})
					return
				}

				event := StreamEvent{
					SessionID: sessionID,
					Content:   delta.Content,
				}

				switch delta.Type {
				case "tool_call_start":
					event.Type = EventToolCall
					event.ToolCalls = delta.ToolCalls
				case "tool_call_end":
					event.Type = EventToolResult
				default:
					event.Type = EventToken
				}

				sm.emit(event)

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (sm *StreamManager) CancelStream(sessionID string) {
	sm.mu.RLock()
	cancel, exists := sm.streams[sessionID]
	sm.mu.RUnlock()
	if exists {
		cancel()
	}
}

func (sm *StreamManager) CancelAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, cancel := range sm.streams {
		cancel()
	}
	sm.streams = make(map[string]context.CancelFunc)
}
