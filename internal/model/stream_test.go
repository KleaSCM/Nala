package model

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewStreamManager(t *testing.T) {
	sm := NewStreamManager()
	if sm == nil {
		t.Fatal("expected stream manager")
	}
}

func TestStreamManager_AddEmitter(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})
}

func TestStreamManager_ConsumeStream_TokenEvents(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch := make(chan StreamDelta, 3)
	ch <- StreamDelta{Type: "token", Content: "Hello"}
	ch <- StreamDelta{Type: "token", Content: " world"}
	ch <- StreamDelta{Done: true, Type: "done"}
	close(ch)

	sm.ConsumeStream(context.Background(), "session-1", ch)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
	if received[0].Type != EventToken {
		t.Errorf("expected EventToken, got %s", received[0].Type)
	}
	if received[0].Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", received[0].Content)
	}
	if received[0].SessionID != "session-1" {
		t.Errorf("expected session-1, got %s", received[0].SessionID)
	}
	if received[1].Content != " world" {
		t.Errorf("expected ' world', got %q", received[1].Content)
	}
	if received[2].Type != EventDone {
		t.Errorf("expected EventDone, got %s", received[2].Type)
	}
	mu.Unlock()
}

func TestStreamManager_ConsumeStream_ToolCall(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch := make(chan StreamDelta, 2)
	ch <- StreamDelta{
		Type: "tool_call_start",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "get_weather",
					Arguments: `{"location":"Tokyo"}`,
				},
			},
		},
	}
	ch <- StreamDelta{Done: true, Type: "done"}
	close(ch)

	sm.ConsumeStream(context.Background(), "session-tc", ch)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0].Type != EventToolCall {
		t.Errorf("expected EventToolCall, got %s", received[0].Type)
	}
	if len(received[0].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(received[0].ToolCalls))
	}
	if received[0].ToolCalls[0].ID != "call_1" {
		t.Errorf("expected call_1, got %s", received[0].ToolCalls[0].ID)
	}
	mu.Unlock()
}

func TestStreamManager_ConsumeStream_Error(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch := make(chan StreamDelta, 1)
	ch <- StreamDelta{Type: "error", Error: "something went wrong"}
	close(ch)

	sm.ConsumeStream(context.Background(), "session-err", ch)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != EventError {
		t.Errorf("expected EventError, got %s", received[0].Type)
	}
	if received[0].Error != "something went wrong" {
		t.Errorf("expected error message, got %q", received[0].Error)
	}
	mu.Unlock()
}

func TestStreamManager_ConsumeStream_UsageOnDone(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch := make(chan StreamDelta, 1)
	ch <- StreamDelta{
		Done: true,
		Type: "done",
		Usage: &TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}
	close(ch)

	sm.ConsumeStream(context.Background(), "session-usage", ch)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != EventDone {
		t.Errorf("expected EventDone, got %s", received[0].Type)
	}
	if received[0].Usage == nil {
		t.Fatal("expected usage")
	}
	if received[0].Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", received[0].Usage.TotalTokens)
	}
	mu.Unlock()
}

func TestStreamManager_CancelStream(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	blocked := make(chan StreamDelta)
	sm.ConsumeStream(context.Background(), "session-cancel", blocked)
	sm.CancelStream("session-cancel")

	// Give the goroutine time to process the cancellation
	// before closing the channel (which would also unblock it).
	time.Sleep(20 * time.Millisecond)
	close(blocked)

	// Only check that the goroutine did NOT process the channel close
	// (which would emit EventDone).  If cancellation happens first,
	// the goroutine returns without emitting anything.
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	for _, e := range received {
		if e.Type == EventDone {
			t.Error("expected stream to be cancelled before receiving done from channel close")
		}
	}
	mu.Unlock()
}

func TestStreamManager_CancelAll(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch1 := make(chan StreamDelta)
	ch2 := make(chan StreamDelta)
	sm.ConsumeStream(context.Background(), "s1", ch1)
	sm.ConsumeStream(context.Background(), "s2", ch2)

	sm.CancelAll()

	time.Sleep(20 * time.Millisecond)
	close(ch1)
	close(ch2)

	time.Sleep(20 * time.Millisecond)
	if !t.Failed() {
	}
}

func TestStreamManager_MultipleEmitters(t *testing.T) {
	sm := NewStreamManager()
	var mu1, mu2 sync.Mutex
	count1, count2 := 0, 0

	sm.AddEmitter(func(event StreamEvent) {
		mu1.Lock()
		count1++
		mu1.Unlock()
	})
	sm.AddEmitter(func(event StreamEvent) {
		mu2.Lock()
		count2++
		mu2.Unlock()
	})

	ch := make(chan StreamDelta, 2)
	ch <- StreamDelta{Type: "token", Content: "test"}
	ch <- StreamDelta{Done: true, Type: "done"}
	close(ch)

	sm.ConsumeStream(context.Background(), "session-multi", ch)
	time.Sleep(50 * time.Millisecond)

	mu1.Lock()
	mu2.Lock()
	if count1 != 2 {
		t.Errorf("expected emitter1 to receive 2 events, got %d", count1)
	}
	if count2 != 2 {
		t.Errorf("expected emitter2 to receive 2 events, got %d", count2)
	}
	mu1.Unlock()
	mu2.Unlock()
}

func TestStreamManager_ConcurrentSessions(t *testing.T) {
	sm := NewStreamManager()
	received := make([]StreamEvent, 0)
	var mu sync.Mutex

	sm.AddEmitter(func(event StreamEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	ch1 := make(chan StreamDelta, 2)
	ch2 := make(chan StreamDelta, 2)
	ch1 <- StreamDelta{Type: "token", Content: "from-1"}
	ch1 <- StreamDelta{Done: true, Type: "done"}
	close(ch1)
	ch2 <- StreamDelta{Type: "token", Content: "from-2"}
	ch2 <- StreamDelta{Done: true, Type: "done"}
	close(ch2)

	sm.ConsumeStream(context.Background(), "s1", ch1)
	sm.ConsumeStream(context.Background(), "s2", ch2)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 4 {
		t.Fatalf("expected 4 events total, got %d", len(received))
	}
	mu.Unlock()
}
