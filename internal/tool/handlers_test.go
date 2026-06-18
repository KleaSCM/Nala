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

func TestWebSearch_Execute_success(t *testing.T) {
	orig := httpDo
	defer func() { httpDo = orig }()
	httpDo = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       toReadCloser(`{"AbstractText":"hello world","AbstractURL":"https://example.com","Results":[{"Text":"result one","FirstURL":"https://a.com"},{"Text":"result two","FirstURL":"https://b.com"}]}`),
		}, nil
	}

	result, err := WebSearch{}.Execute(context.Background(), json.RawMessage(`{"query":"test","max_results":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if !strings.Contains(result.Content, "hello world") {
		t.Fatalf("expected abstract text, got %q", result.Content)
	}
	if strings.Contains(result.Content, "result two") {
		t.Fatal("max_results not respected")
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

	result, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`{"url":"http://example.com","max_chars":10}`))
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

	result, err := WebFetch{}.Execute(context.Background(), json.RawMessage(`{"url":"http://example.com"}`))
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

func toReadCloser(s string) *rc {
	return &rc{strings.NewReader(s)}
}

type rc struct{ *strings.Reader }

func (*rc) Close() error { return nil }

func TestListTools(t *testing.T) {
	reg := NewRegistry()
	tools := []Tool{
		WebSearch{},
		WebFetch{},
		FileRead{SandboxDir: t.TempDir()},
		FileWrite{SandboxDir: t.TempDir()},
		FileList{SandboxDir: t.TempDir()},
		FileDelete{SandboxDir: t.TempDir()},
		ShellRun{},
	}
	reg.RegisterMany(tools...)

	all := reg.All()
	if len(all) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(all))
	}

	cats := reg.Categories()
	if len(cats) == 0 {
		t.Fatal("expected categories")
	}
}
