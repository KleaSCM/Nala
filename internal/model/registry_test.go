/**
 * Tests for provider registry operations.
 * プロバイダレジストリ操作のテストね。
 *
 * Covers register, get, list, remove, duplicate error,
 * and nonexistent provider lookup.
 * 登録、取得、一覧、削除、重複エラー、存在しないプロバイダの検索をカバーしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package model

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{}

	err := r.Register(p)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	got, err := r.Get("mock")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID() != "mock" {
		t.Errorf("expected mock, got %s", got.ID())
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{}

	if err := r.Register(p); err != nil {
		t.Fatalf("first Register error: %v", err)
	}
	if err := r.Register(p); err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestRegistryGetNonexistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{})

	providers := r.List()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{})
	r.Remove("mock")

	if r.Len() != 0 {
		t.Errorf("expected 0 providers after remove, got %d", r.Len())
	}
}

func TestRegistryChat(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{})

	resp, err := r.Chat(context.Background(), "mock", ChatRequest{})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Provider != "mock" {
		t.Errorf("expected mock, got %s", resp.Provider)
	}
}

func TestRegistryChatStream(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{})

	ch, err := r.ChatStream(context.Background(), "mock", ChatRequest{})
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}
	delta := <-ch
	if !delta.Done {
		t.Error("expected done delta")
	}
}

func TestRegistryChatNonexistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Chat(context.Background(), "ghost", ChatRequest{})
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestRegistryEmptyList(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Errorf("expected 0, got %d", r.Len())
	}
	if len(r.List()) != 0 {
		t.Error("expected empty list")
	}
}
