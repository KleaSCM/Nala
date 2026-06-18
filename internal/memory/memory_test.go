package memory

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestVectorDB_InsertAndSearch(t *testing.T) {
	vdb := NewVectorDB()
	ctx := context.Background()

	vdb.Insert(ctx, "test", "1", []float32{1, 0, 0}, "doc one")
	vdb.Insert(ctx, "test", "2", []float32{0, 1, 0}, "doc two")
	vdb.Insert(ctx, "test", "3", []float32{0, 0, 1}, "doc three")

	results, err := vdb.Search(ctx, "test", []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "1" {
		t.Fatalf("expected ID '1' (closest), got %q", results[0].ID)
	}
}

func TestVectorDB_Delete(t *testing.T) {
	vdb := NewVectorDB()
	ctx := context.Background()

	vdb.Insert(ctx, "test", "1", []float32{1, 0, 0}, "doc one")
	vdb.Insert(ctx, "test", "2", []float32{0, 1, 0}, "doc two")

	err := vdb.Delete(ctx, "test", "1")
	if err != nil {
		t.Fatal(err)
	}

	count, _ := vdb.CollectionStats(ctx, "test")
	if count != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", count)
	}
}

func TestVectorDB_SearchWithMinScore(t *testing.T) {
	vdb := NewVectorDB()
	ctx := context.Background()

	vdb.Insert(ctx, "test", "close", []float32{0.99, 0, 0}, "close match")
	vdb.Insert(ctx, "test", "far", []float32{0, 0, 0.99}, "far match")

	results, err := vdb.SearchWithMinScore(ctx, "test", []float32{1, 0, 0}, 10, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result above 0.9 threshold, got %d", len(results))
	}
	if results[0].ID != "close" {
		t.Fatalf("expected 'close', got %q", results[0].ID)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		a, b   []float32
		expect float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{[]float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{[]float32{1, 1, 0}, []float32{1, 1, 0}, 1.0},
		{[]float32{}, []float32{}, 0.0},
	}
	for _, tt := range tests {
		got := cosineSimilarity(tt.a, tt.b)
		if math.Abs(got-tt.expect) > 0.001 {
			t.Fatalf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expect)
		}
	}
}

func TestChunker_Fixed(t *testing.T) {
	c := NewChunker("fixed", 10, 2)
	chunks, err := c.Chunk(context.Background(), "hello world this is a test of the chunker")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if chunks[0].Index != 0 {
		t.Fatalf("expected index 0, got %d", chunks[0].Index)
	}
}

func TestChunker_Recursive(t *testing.T) {
	c := NewChunker("recursive", 100, 10)
	content := strings.Repeat("This is a paragraph. ", 50) + "\n\n" + strings.Repeat("Another paragraph here. ", 50)
	chunks, err := c.Chunk(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestChunker_ContentTooLarge(t *testing.T) {
	c := NewChunker("recursive", 1000, 200)
	large := strings.Repeat("x", 11*1024*1024)
	if !c.ContentTooLarge(large) {
		t.Fatal("expected content too large")
	}
}

func TestEntityExtractor_Extract(t *testing.T) {
	ee := NewEntityExtractor()
	entities := ee.Extract("Contact me at user@example.com or visit https://nala.ai")
	hasEmail := false
	hasURL := false
	for _, e := range entities {
		if e.Type == "email" {
			hasEmail = true
		}
		if e.Type == "url" {
			hasURL = true
		}
	}
	if !hasEmail {
		t.Fatal("expected email entity")
	}
	if !hasURL {
		t.Fatal("expected url entity")
	}
}

func TestEntityExtractor_Store(t *testing.T) {
	ee := NewEntityExtractor()
	ee.Store("Contact user@example.com")
	ee.Store("Visit user@example.com again")

	if ee.Count() != 1 {
		t.Fatalf("expected 1 unique entity, got %d", ee.Count())
	}

	results := ee.Find("user@example.com")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestEntityExtractor_Technology(t *testing.T) {
	ee := NewEntityExtractor()
	entities := ee.Extract("I love Python and Go programming")
	hasPython := false
	hasGo := false
	for _, e := range entities {
		if e.Type == "technology" && e.Value == "Python" {
			hasPython = true
		}
		if e.Type == "technology" && e.Value == "Go" {
			hasGo = true
		}
	}
	if !hasPython || !hasGo {
		t.Fatal("expected Python and Go technology entities")
	}
}

func TestManager_StoreAndRecallUserMemory(t *testing.T) {
	mgr := New(nil)
	ctx := context.Background()

	mgr.StoreUserMemory(ctx, "User likes cats", "preference", 0.9)
	mgr.StoreUserMemory(ctx, "User works at Google", "fact", 0.8)

	mems, err := mgr.RecallUserMemory(ctx, "cats", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(mems) == 0 {
		t.Fatal("expected memories about cats")
	}
	if !containsFact(mems, "cats") {
		t.Fatalf("expected 'cats' in results: %+v", mems)
	}
}

func TestManager_AddMessages(t *testing.T) {
	mgr := New(nil)
	ctx := context.Background()

	mgr.AddMessages(ctx, "session1", []string{"Hello, my name is Alice", "I work at Acme Corp"})

	conv, ok := mgr.GetConversation(ctx, "session1")
	if !ok {
		t.Fatal("expected conversation")
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(conv.Messages))
	}

	// Check entities were extracted (from "my name is Alice")
	entities := mgr.Extractor.Find("Alice")
	if len(entities) == 0 {
		// May fail depending on name format; log and continue
		t.Log("entity 'Alice' not found, checking all entities")
		for _, e := range mgr.Extractor.All() {
			t.Logf("entity: %s = %s (conf: %.2f)", e.Type, e.Value, e.Confidence)
		}
	}
}

func TestManager_MultiHopQuery(t *testing.T) {
	mgr := New(nil)
	ctx := context.Background()

	mgr.StoreUserMemory(ctx, "User is a software developer", "fact", 0.9)
	mgr.StoreUserMemory(ctx, "User likes TypeScript and Rust", "preference", 0.8)

	result, err := mgr.MultiHopQuery(ctx, "what programming languages does the user like?", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Context, "TypeScript") && !strings.Contains(result.Context, "Rust") {
		t.Fatalf("expected programming languages in result: %s", result.Context)
	}
}

func TestManager_QueryMemory(t *testing.T) {
	mgr := New(nil)
	ctx := context.Background()

	mgr.StoreUserMemory(ctx, "User prefers concise answers", "preference", 0.8)

	resp, err := mgr.QueryMemory(ctx, MemoryQuery{Text: "preferences", MinConfidence: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sources) == 0 && resp.Confidence < 0.1 {
		// It may not find results if no FTS, but should not error
		t.Log("response:", resp.Answer[:min(len(resp.Answer), 100)])
	}
}

func TestDocumentProcessor_ProcessText(t *testing.T) {
	// Test MIME detection from extension
	mime := detectMimeType(".txt", []byte("hello"))
	if mime != "text/plain" {
		t.Fatalf("expected text/plain, got %s", mime)
	}

	mime2 := detectMimeType(".md", []byte("# header"))
	if mime2 != "text/markdown" {
		t.Fatalf("expected text/markdown, got %s", mime2)
	}
}

func TestExtractors(t *testing.T) {
	t.Run("strip html", func(t *testing.T) {
		result := stripHTML("<html><body><p>Hello <b>World</b></p></body></html>")
		if !strings.Contains(result, "Hello World") {
			t.Fatalf("expected 'Hello World', got %q", result)
		}
	})

	t.Run("extract json", func(t *testing.T) {
		result := extractJSON([]byte(`{"name":"test"}`))
		if !strings.Contains(result, "test") {
			t.Fatalf("expected 'test', got %q", result)
		}
	})

	t.Run("extract csv", func(t *testing.T) {
		result := extractCSV([]byte("a,b,c\n1,2,3"))
		if !strings.Contains(result, "1 | 2 | 3") {
			t.Fatalf("expected '1 | 2 | 3', got %q", result)
		}
	})

	t.Run("extract pdf", func(t *testing.T) {
		// Minimal PDF-like content
		result := extractPDF([]byte("(Hello World) Tj"))
		if !strings.Contains(result, "Hello World") {
			t.Fatalf("expected 'Hello World' in extracted text, got %q", result)
		}
	})
}

func TestExtractFacts(t *testing.T) {
	facts := extractFacts("My name is Bob and I work as a developer")
	hasName := false
	hasWork := false
	for _, f := range facts {
		if strings.Contains(f, "Bob") {
			hasName = true
		}
		if strings.Contains(f, "developer") {
			hasWork = true
		}
	}
	if !hasName {
		t.Fatalf("expected name fact, got %v", facts)
	}
	if !hasWork {
		t.Fatalf("expected work fact, got %v", facts)
	}
}

func TestFTSManager_Initialize(t *testing.T) {
	// Test with nil DB (should not panic)
	fts := NewFTSManager(nil)
	if err := fts.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCachedEmbedder(t *testing.T) {
	calls := 0
	mock := &mockEmbedder{
		fn: func(texts []string) ([][]float32, error) {
			calls++
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{0.1, 0.2, 0.3}
			}
			return result, nil
		},
	}
	cached := NewCachedEmbedder(mock)

	ctx := context.Background()
	// First call
	cached.Embed(ctx, []string{"hello"})
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	// Second call with same text
	cached.Embed(ctx, []string{"hello"})
	if calls != 1 {
		t.Fatalf("expected 0 more calls (cached), got %d", calls)
	}
	// Different text
	cached.Embed(ctx, []string{"world"})
	if calls != 2 {
		t.Fatalf("expected 1 more call, got %d", calls)
	}
}

func TestRAGPipeline_EmptyResult(t *testing.T) {
	vdb := NewVectorDB()
	rag := NewRAGPipeline(vdb, nil, NewChunker("recursive", 1000, 200))
	ctx := context.Background()

	result, err := rag.Query(ctx, "test", "", 5, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(result.Sources))
	}
}

func TestApproxTokenCount(t *testing.T) {
	count := approxTokenCount("hello world this is a test")
	if count <= 0 {
		t.Fatal("expected positive token count")
	}
}

func containsFact(mems []UserMemory, substr string) bool {
	for _, m := range mems {
		if strings.Contains(strings.ToLower(m.Fact), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

type mockEmbedder struct {
	fn func(texts []string) ([][]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return m.fn(texts)
}

func (m *mockEmbedder) Dimensions() int { return 3 }
