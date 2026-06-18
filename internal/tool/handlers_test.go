package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebSearch_ValidateArgs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := WebSearch{}.ValidateArgs(json.RawMessage(`{"query":"hello"}`))
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
	t.Run("missing query", func(t *testing.T) {
		err := WebSearch{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWebSearch_Execute_networkError(t *testing.T) {
	orig := httpDo
	defer func() { httpDo = orig }()
	httpDo = func(*http.Request) (*http.Response, error) {
		return nil, os.ErrClosed
	}

	result, err := WebSearch{}.Execute(context.Background(), json.RawMessage(`{"query":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Web search unavailable" {
		t.Fatalf("expected 'Web search unavailable', got %q", result.Content)
	}
}

func TestWebSearch_Execute_emptyQuery(t *testing.T) {
	result, err := WebSearch{}.Execute(context.Background(), json.RawMessage(`{"query":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "No results found" {
		t.Fatalf("expected 'No results found', got %q", result.Content)
	}
}

func TestWebSearch_Cache(t *testing.T) {
	orig := httpDo
	defer func() { httpDo = orig }()
	callCount := 0
	httpDo = func(*http.Request) (*http.Response, error) {
		callCount++
		resp := &http.Response{
			StatusCode: 200,
			Body:       toReadCloser(`{"AbstractText":"cached test","AbstractURL":"https://example.com","Results":[]}`),
		}
		return resp, nil
	}

	// First call — should hit API
	WebSearch{}.Execute(context.Background(), json.RawMessage(`{"query":"cache_test"}`))
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Second call — should use cache
	WebSearch{}.Execute(context.Background(), json.RawMessage(`{"query":"cache_test"}`))
	if callCount != 1 {
		t.Fatalf("expected 0 more API calls (cached), got %d", callCount)
	}
}

func TestWebFetch_ValidateArgs(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		err := WebFetch{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWebFetch_Execute_success(t *testing.T) {
	orig := httpDo
	defer func() { httpDo = orig }()
	httpDo = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       toReadCloser("hello world this is a long response body"),
		}, nil
	}

	result, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`{"url":"http://unique-test.local/fetch","max_chars":10}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("expected truncated")
	}
	if len(result.Content) > 10 {
		t.Fatalf("expected max 10 chars, got %d", len(result.Content))
	}
}

func TestWebFetch_Execute_networkError(t *testing.T) {
	orig := httpDo
	defer func() { httpDo = orig }()
	httpDo = func(*http.Request) (*http.Response, error) {
		return nil, os.ErrClosed
	}

	// Use unique URL to avoid cache from other tests
	result, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`{"url":"http://network-error-test.local/test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Web fetch unavailable" {
		t.Fatalf("expected 'Web fetch unavailable', got %q", result.Content)
	}
}

func TestFileRead_SandboxPath(t *testing.T) {
	td := t.TempDir()

	t.Run("valid path", func(t *testing.T) {
		os.WriteFile(filepath.Join(td, "test.txt"), []byte("hello"), 0644)
		fr := FileRead{SandboxDir: td}
		result, err := fr.Execute(context.Background(), json.RawMessage(`{"path":"test.txt"}`))
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "hello" {
			t.Fatalf("expected 'hello', got %q", result.Content)
		}
	})

	t.Run("traversal blocked", func(t *testing.T) {
		fr := FileRead{SandboxDir: td}
		result, err := fr.Execute(context.Background(), json.RawMessage(`{"path":"../etc/passwd"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "outside sandbox") {
			t.Fatalf("expected sandbox error, got %q", result.Content)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		fr := FileRead{SandboxDir: td}
		result, err := fr.Execute(context.Background(), json.RawMessage(`{"path":"nonexistent.txt"}`))
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "File not found" {
			t.Fatalf("expected 'File not found', got %q", result.Content)
		}
	})

	t.Run("absolute path blocked", func(t *testing.T) {
		fr := FileRead{SandboxDir: td}
		result, err := fr.Execute(context.Background(), json.RawMessage(`{"path":"/etc/passwd"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "outside sandbox") {
			t.Fatalf("expected sandbox error, got %q", result.Content)
		}
	})

	t.Run("binary file", func(t *testing.T) {
		binary := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		os.WriteFile(filepath.Join(td, "binary.bin"), binary, 0644)
		fr := FileRead{SandboxDir: td}
		result, err := fr.Execute(context.Background(), json.RawMessage(`{"path":"binary.bin"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "Binary file") {
			t.Fatalf("expected binary file msg, got %q", result.Content)
		}
	})
}

func TestFileWrite(t *testing.T) {
	td := t.TempDir()
	fw := FileWrite{SandboxDir: td}

	t.Run("write new file", func(t *testing.T) {
		result, err := fw.Execute(context.Background(), json.RawMessage(`{"path":"new.txt","content":"hello"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		data, _ := os.ReadFile(filepath.Join(td, "new.txt"))
		if string(data) != "hello" {
			t.Fatalf("expected 'hello', got %q", string(data))
		}
	})

	t.Run("append to file", func(t *testing.T) {
		result, err := fw.Execute(context.Background(), json.RawMessage(`{"path":"new.txt","content":" world","mode":"append"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		data, _ := os.ReadFile(filepath.Join(td, "new.txt"))
		if string(data) != "hello world" {
			t.Fatalf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("create_new rejects existing", func(t *testing.T) {
		result, err := fw.Execute(context.Background(), json.RawMessage(`{"path":"new.txt","content":"again","mode":"create_new"}`))
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "File already exists" {
			t.Fatalf("expected 'File already exists', got %q", result.Content)
		}
	})

	t.Run("traversal blocked", func(t *testing.T) {
		result, err := fw.Execute(context.Background(), json.RawMessage(`{"path":"../../outside.txt","content":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "outside sandbox") {
			t.Fatalf("expected sandbox error, got %q", result.Content)
		}
	})
}

func TestFileList(t *testing.T) {
	td := t.TempDir()
	os.WriteFile(filepath.Join(td, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(td, "b.txt"), []byte("bb"), 0644)
	os.MkdirAll(filepath.Join(td, "sub"), 0755)

	fl := FileList{SandboxDir: td}
	result, err := fl.Execute(context.Background(), json.RawMessage(`{"path":"."}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "a.txt") {
		t.Fatalf("expected 'a.txt', got %q", result.Content)
	}
	if !strings.Contains(result.Content, "b.txt") {
		t.Fatalf("expected 'b.txt', got %q", result.Content)
	}
	if !strings.Contains(result.Content, "sub/") {
		t.Fatalf("expected 'sub/', got %q", result.Content)
	}
}

func TestFileList_RecursiveAndPattern(t *testing.T) {
	td := t.TempDir()
	os.MkdirAll(filepath.Join(td, "sub"), 0755)
	os.WriteFile(filepath.Join(td, "sub", "c.txt"), []byte("c"), 0644)
	os.WriteFile(filepath.Join(td, "data.json"), []byte("{}"), 0644)

	fl := FileList{SandboxDir: td}

	t.Run("recursive", func(t *testing.T) {
		result, err := fl.Execute(context.Background(), json.RawMessage(`{"path":".","recursive":true}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "c.txt") {
			t.Fatalf("expected 'c.txt', got %q", result.Content)
		}
	})

	t.Run("pattern filters", func(t *testing.T) {
		result, err := fl.Execute(context.Background(), json.RawMessage(`{"path":".","pattern":"*.json"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "data.json") {
			t.Fatalf("expected 'data.json', got %q", result.Content)
		}
		if strings.Contains(result.Content, "a.txt") {
			t.Fatal("pattern should filter out .txt files")
		}
	})
}

func TestFileDelete(t *testing.T) {
	td := t.TempDir()
	os.WriteFile(filepath.Join(td, "del.txt"), []byte("x"), 0644)

	fd := FileDelete{SandboxDir: td}
	result, err := fd.Execute(context.Background(), json.RawMessage(`{"path":"del.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Moved to trash" {
		t.Fatalf("expected 'Moved to trash', got %q", result.Content)
	}
	if _, err := os.Stat(filepath.Join(td, ".trash", "del.txt")); err != nil {
		t.Fatal("expected file in trash")
	}

	t.Run("permanent delete", func(t *testing.T) {
		os.WriteFile(filepath.Join(td, "perm.txt"), []byte("x"), 0644)
		result, err := fd.Execute(context.Background(), json.RawMessage(`{"path":"perm.txt","permanent":true}`))
		if err != nil {
			t.Fatal(err)
		}
		if result.Content != "Permanently deleted" {
			t.Fatalf("expected 'Permanently deleted', got %q", result.Content)
		}
		if _, err := os.Stat(filepath.Join(td, "perm.txt")); !os.IsNotExist(err) {
			t.Fatal("expected file to be gone")
		}
	})
}

func TestShellRun(t *testing.T) {
	t.Run("whitelisted command", func(t *testing.T) {
		result, err := ShellRun{}.Execute(context.Background(), json.RawMessage(`{"command":"echo","args":["hello"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if strings.TrimSpace(result.Content) != "hello" {
			t.Fatalf("expected 'hello', got %q", strings.TrimSpace(result.Content))
		}
	})

	t.Run("non-whitelisted rejected", func(t *testing.T) {
		result, err := ShellRun{}.Execute(context.Background(), json.RawMessage(`{"command":"rm","args":["-rf","/"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "not whitelisted") {
			t.Fatalf("expected whitelist error, got %q", result.Content)
		}
	})

	t.Run("chaining blocked", func(t *testing.T) {
		result, err := ShellRun{}.Execute(context.Background(), json.RawMessage(`{"command":"echo","args":["hello; rm -rf /"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "not allowed") {
			t.Fatalf("expected chaining error, got %q", result.Content)
		}
	})

	t.Run("validate missing command", func(t *testing.T) {
		err := ShellRun{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCodeExecute(t *testing.T) {
	t.Run("python hello", func(t *testing.T) {
		result, err := CodeExecute{}.Execute(context.Background(), json.RawMessage(`{"language":"python","code":"print('hello from nala')"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if !strings.Contains(result.Content, "hello from nala") {
			t.Fatalf("expected 'hello from nala', got %q", result.Content)
		}
	})

	t.Run("bash echo", func(t *testing.T) {
		result, err := CodeExecute{}.Execute(context.Background(), json.RawMessage(`{"language":"bash","code":"echo hello bash"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
	})

	t.Run("disallowed language", func(t *testing.T) {
		result, err := CodeExecute{}.Execute(context.Background(), json.RawMessage(`{"language":"rust","code":"fn main() {}"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "not allowed") {
			t.Fatalf("expected 'not allowed', got %q", result.Content)
		}
	})

	t.Run("validate args", func(t *testing.T) {
		err := CodeExecute{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDBQuery_InsertBlocked(t *testing.T) {
	dq := DBQuery{}
	result, err := dq.Execute(context.Background(), json.RawMessage(`{"query":"INSERT INTO test VALUES (1, 'x')"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "read-only") && !strings.Contains(result.Content, "not available") {
		t.Fatalf("expected 'read-only' or 'not available', got %q", result.Content)
	}
}

func TestDBQuery_DBNotAvailable(t *testing.T) {
	dq := DBQuery{DB: nil}
	result, err := dq.Execute(context.Background(), json.RawMessage(`{"query":"SELECT 1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "not available") {
		t.Fatalf("expected 'not available', got %q", result.Content)
	}
}

func TestDBQuery_ValidateArgs(t *testing.T) {
	err := DBQuery{}.ValidateArgs(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPRequest_PrivateBlocked(t *testing.T) {
	h := HTTPRequest{}
	result, err := h.Execute(context.Background(), json.RawMessage(`{"url":"http://127.0.0.1:8080/admin"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "blocked") {
		t.Fatalf("expected 'blocked', got %q", result.Content)
	}
}

func TestHTTPRequest_ValidateArgs(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		err := HTTPRequest{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMemoryStore(t *testing.T) {
	t.Run("store fact", func(t *testing.T) {
		m := MemoryStore{}
		result, err := m.Execute(context.Background(), json.RawMessage(`{"fact":"User likes cats"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
	})

	t.Run("validate args", func(t *testing.T) {
		err := MemoryStore{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMemoryRecall(t *testing.T) {
	t.Run("recall with store fn", func(t *testing.T) {
		m := MemoryRecall{RecallFn: func(ctx context.Context, query string, topK int) ([]MemoryResult, error) {
			return []MemoryResult{{Fact: "User likes cats", Category: "preference", Confidence: 0.9, Source: "explicit"}}, nil
		}}
		result, err := m.Execute(context.Background(), json.RawMessage(`{"query":"likes","top_k":5}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if !strings.Contains(result.Content, "cats") {
			t.Fatalf("expected 'cats', got %q", result.Content)
		}
	})

	t.Run("recall without store fn", func(t *testing.T) {
		m := MemoryRecall{}
		result, err := m.Execute(context.Background(), json.RawMessage(`{"query":"default"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
	})
}

func TestKnowledgeSearch(t *testing.T) {
	t.Run("with embed and search fns", func(t *testing.T) {
		k := KnowledgeSearch{
			EmbedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
				return [][]float32{{0.1, 0.2, 0.3}}, nil
			},
			SearchFn: func(ctx context.Context, collection string, vector []float32, topK int, minScore float64) ([]VectorResult, error) {
				return []VectorResult{{ID: "doc1", Score: 0.95, Metadata: "Test document about AI"}}, nil
			},
		}
		result, err := k.Execute(context.Background(), json.RawMessage(`{"query":"AI","top_k":5,"min_score":0.7}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if !strings.Contains(result.Content, "AI") {
			t.Fatalf("expected 'AI', got %q", result.Content)
		}
	})

	t.Run("without fns", func(t *testing.T) {
		k := KnowledgeSearch{}
		result, err := k.Execute(context.Background(), json.RawMessage(`{"query":"test"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "embedding provider") {
			t.Fatalf("expected 'embedding provider', got %q", result.Content)
		}
	})
}

func TestAURSearch(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := AURSearch{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAURInfo(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := AURInfo{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAURInstall(t *testing.T) {
	t.Run("no packages specified", func(t *testing.T) {
		result, err := AURInstall{}.Execute(context.Background(), json.RawMessage(`{"packages":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "No packages") {
			t.Fatalf("expected 'No packages', got %q", result.Content)
		}
	})
}

func TestImageGenerate(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := ImageGenerate{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("sd not running", func(t *testing.T) {
		orig := httpDo
		defer func() { httpDo = orig }()
		httpDo = func(*http.Request) (*http.Response, error) {
			return nil, os.ErrClosed
		}
		result, err := ImageGenerate{}.Execute(context.Background(), json.RawMessage(`{"prompt":"a cat"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "unavailable") {
			t.Fatalf("expected 'unavailable', got %q", result.Content)
		}
	})
}

func TestImageAnalyze(t *testing.T) {
	result, err := ImageAnalyze{}.Execute(context.Background(), json.RawMessage(`{"url":"https://example.com/img.png","prompt":"describe"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "vision") {
		t.Fatalf("expected vision info, got %q", result.Content)
	}
}

func TestCalendarList(t *testing.T) {
	result, err := CalendarList{}.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestCalendarCreate(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := CalendarCreate{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestEmailSend(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := EmailSend{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestEmailInbox(t *testing.T) {
	result, err := EmailInbox{}.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestNotesCreate(t *testing.T) {
	td := t.TempDir()
	n := NotesCreate{NotesDir: td}

	t.Run("create note", func(t *testing.T) {
		result, err := n.Execute(context.Background(), json.RawMessage(`{"title":"Test Note","content":"Hello world","tags":["test"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if !strings.Contains(result.Content, "Test Note") {
			t.Fatalf("expected 'Test Note', got %q", result.Content)
		}
	})

	t.Run("folder traversal blocked", func(t *testing.T) {
		result, err := n.Execute(context.Background(), json.RawMessage(`{"title":"X","content":"X","folder":"../../etc"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "blocked") && !strings.Contains(result.Content, "Invalid") {
			t.Fatalf("expected error, got %q", result.Content)
		}
	})
}

func TestNotesList(t *testing.T) {
	td := t.TempDir()
	n := NotesCreate{NotesDir: td}
	n.Execute(context.Background(), json.RawMessage(`{"title":"ListTest","content":"content"}`))

	l := NotesList{NotesDir: td}
	result, err := l.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "ListTest") {
		t.Fatalf("expected 'ListTest', got %q", result.Content)
	}
}

func TestFileSearch(t *testing.T) {
	td := t.TempDir()
	os.WriteFile(filepath.Join(td, "report.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(td, "data.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(td, "sub"), 0755)
	os.WriteFile(filepath.Join(td, "sub", "notes.txt"), []byte("x"), 0644)

	fs := FileSearch{SandboxDir: td}

	t.Run("glob pattern", func(t *testing.T) {
		result, err := fs.Execute(context.Background(), json.RawMessage(`{"pattern":"*.txt","root":"."}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "report.txt") {
			t.Fatalf("expected 'report.txt', got %q", result.Content)
		}
	})

	t.Run("traversal blocked", func(t *testing.T) {
		result, err := fs.Execute(context.Background(), json.RawMessage(`{"pattern":"*","root":"../etc"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "outside sandbox") {
			t.Fatalf("expected sandbox error, got %q", result.Content)
		}
	})

	t.Run("recursive search", func(t *testing.T) {
		result, err := fs.Execute(context.Background(), json.RawMessage(`{"pattern":"*.txt","root":".","recursive":true}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Content, "notes.txt") {
			t.Fatalf("expected 'notes.txt', got %q", result.Content)
		}
	})
}

func TestSystemMonitor(t *testing.T) {
	result, err := SystemMonitor{}.Execute(context.Background(), json.RawMessage(`{"metric":"cpu"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestSystemProcesses(t *testing.T) {
	result, err := SystemProcesses{}.Execute(context.Background(), json.RawMessage(`{"limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestSystemLogs(t *testing.T) {
	result, err := SystemLogs{}.Execute(context.Background(), json.RawMessage(`{"limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	// Should have some content (may be "no log" message in test env)
	if result.Content == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestSystemNotify(t *testing.T) {
	t.Run("validate args", func(t *testing.T) {
		err := SystemNotify{}.ValidateArgs(json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSandboxPath(t *testing.T) {
	t.Run("simple path", func(t *testing.T) {
		abs, err := sandboxPath("/tmp/sandbox", "test.txt")
		if err != nil {
			t.Fatal(err)
		}
		if abs != "/tmp/sandbox/test.txt" {
			t.Fatalf("unexpected path: %s", abs)
		}
	})
	t.Run("traversal blocked", func(t *testing.T) {
		_, err := sandboxPath("/tmp/sandbox", "../etc/passwd")
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("absolute blocked", func(t *testing.T) {
		_, err := sandboxPath("/tmp/sandbox", "/etc/passwd")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestListTools(t *testing.T) {
	reg := NewRegistry()
	tools := []Tool{
		WebSearch{},
		WebFetch{},
		FileRead{SandboxDir: t.TempDir()},
		FileWrite{SandboxDir: t.TempDir()},
		FileList{SandboxDir: t.TempDir()},
		FileDelete{SandboxDir: t.TempDir()},
		FileSearch{SandboxDir: t.TempDir()},
		ShellRun{},
		CodeExecute{},
		HTTPRequest{},
		ImageGenerate{},
		ImageAnalyze{},
		KnowledgeSearch{},
		MemoryStore{},
		MemoryRecall{},
		CalendarList{},
		CalendarCreate{},
		EmailSend{},
		EmailInbox{},
		NotesCreate{},
		NotesList{},
		AURSearch{},
		AURInfo{},
		AURInstall{},
		AURRemove{},
		AURUpdate{},
		AURList{},
		SystemMonitor{},
		SystemProcesses{},
		SystemLogs{},
		SystemNotify{},
	}
	reg.RegisterMany(tools...)
	all := reg.All()
	if len(all) != 31 {
		t.Fatalf("expected 31 tools, got %d", len(all))
	}
	cats := reg.Categories()
	if len(cats) == 0 {
		t.Fatal("expected categories")
	}
}

func toReadCloser(s string) *rc {
	return &rc{strings.NewReader(s)}
}

type rc struct{ *strings.Reader }

func (*rc) Close() error { return nil }
