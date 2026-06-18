/**
 * Provider registry — register, look up, and enumerate LLM providers.
 * プロバイダレジストリ — LLMプロバイダの登録、検索、列挙をするの。
 *
 * Thread-safe with RWMutex. Used by the router to find providers
 * by ID and list all available providers for the UI.
 * RWMutexでスレッドセーフになってるの。ルーターがプロバイダをIDで検索したり、
 * UIに全プロバイダを表示したりするのに使うわ。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"fmt"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

func (r *Registry) Register(p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := p.ID()
	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("model: provider %q already registered", id)
	}
	r.providers[id] = p
	return nil
}

func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.providers[id]
	if !exists {
		return nil, fmt.Errorf("model: provider %q not found: %w", id, ErrProviderNotFound)
	}
	return p, nil
}

func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

func (r *Registry) Chat(ctx context.Context, providerID string, req ChatRequest) (*ChatResponse, error) {
	p, err := r.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.Chat(ctx, req)
}

func (r *Registry) ChatStream(ctx context.Context, providerID string, req ChatRequest) (<-chan StreamDelta, error) {
	p, err := r.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.ChatStream(ctx, req)
}
