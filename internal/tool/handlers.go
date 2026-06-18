package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

var httpDo = http.DefaultClient.Do

// ── web.search ───────────────────────────────────────────────────────

type WebSearch struct{}

func (WebSearch) ID() string                     { return "web.search" }
func (WebSearch) Name() string                   { return "Web Search" }
func (WebSearch) Description() string            { return "Search the web using DuckDuckGo" }
func (WebSearch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"max_results": {"type": "integer", "default": 5}
		},
		"required": ["query"]
	}`)
}

type webSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func (WebSearch) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p webSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("web.search: invalid args: %w", err)
	}
	if p.Query == "" {
		return &Result{Content: "No results found"}, nil
	}
	if p.MaxResults <= 0 || p.MaxResults > 20 {
		p.MaxResults = 5
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(p.Query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("web.search: request: %w", err)
	}
	req.Header.Set("User-Agent", "Nala/1.0")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "Web search unavailable"}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var ddg struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Results      []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}
	json.Unmarshal(body, &ddg)

	var b strings.Builder
	if ddg.AbstractText != "" {
		b.WriteString(fmt.Sprintf("**%s**\n%s\n\n", ddg.AbstractURL, ddg.AbstractText))
	}
	count := 0
	for _, r := range ddg.Results {
		if count >= p.MaxResults {
			break
		}
		b.WriteString(fmt.Sprintf("- %s\n  %s\n\n", r.Text, r.FirstURL))
		count++
	}
	if b.Len() == 0 {
		b.WriteString("No results found")
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/markdown"}, nil
}

func (WebSearch) ValidateArgs(args json.RawMessage) error {
	var p webSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// ── web.fetch ────────────────────────────────────────────────────────

type WebFetch struct{}

func (WebFetch) ID() string          { return "web.fetch" }
func (WebFetch) Name() string        { return "Web Fetch" }
func (WebFetch) Description() string { return "Fetch a URL and return its content as text" }

func (WebFetch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to fetch"},
			"max_chars": {"type": "integer", "default": 10000}
		},
		"required": ["url"]
	}`)
}

type webFetchArgs struct {
	URL      string `json:"url"`
	MaxChars int    `json:"max_chars"`
}

func (WebFetch) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p webFetchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("web.fetch: invalid args: %w", err)
	}
	if p.URL == "" {
		return &Result{Content: "No URL provided"}, nil
	}
	if p.MaxChars <= 0 || p.MaxChars > 100000 {
		p.MaxChars = 10000
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("web.fetch: request: %w", err)
	}
	req.Header.Set("User-Agent", "Nala/1.0")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "Web fetch unavailable"}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	truncated := false
	if len(content) > p.MaxChars {
		content = content[:p.MaxChars]
		truncated = true
	}

	return &Result{Success: true, Content: content, MimeType: "text/plain", Truncated: truncated}, nil
}

func (WebFetch) ValidateArgs(args json.RawMessage) error {
	var p webFetchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

// ── file.read ────────────────────────────────────────────────────────

type FileRead struct {
	SandboxDir string
}

func (f FileRead) ID() string          { return "file.read" }
func (f FileRead) Name() string        { return "Read File" }
func (f FileRead) Description() string { return "Read a file from the sandbox directory" }

func (f FileRead) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path relative to sandbox"}
		},
		"required": ["path"]
	}`)
}

type fileReadArgs struct {
	Path string `json:"path"`
}

func (f FileRead) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p fileReadArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("file.read: invalid args: %w", err)
	}
	full, err := f.sandboxPath(p.Path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	info, err := os.Stat(full)
	if err != nil {
		return &Result{Content: "File not found"}, nil
	}
	if info.IsDir() {
		return &Result{Content: "Path is a directory"}, nil
	}
	if info.Size() > 10*1024*1024 {
		return &Result{Content: "File too large (max 10MB)"}, nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading file: %v", err)}, nil
	}
	return &Result{Success: true, Content: string(data), MimeType: "text/plain"}, nil
}

func (f FileRead) ValidateArgs(args json.RawMessage) error {
	var p fileReadArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}

func sandboxPath(sandboxDir, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("path outside sandbox")
	}
	full := filepath.Join(sandboxDir, clean)
	abs, _ := filepath.Abs(full)
	sbAbs, _ := filepath.Abs(sandboxDir)
	if !strings.HasPrefix(abs, sbAbs) {
		return "", fmt.Errorf("path outside sandbox")
	}
	return abs, nil
}

func (f FileRead) sandboxPath(relative string) (string, error) {
	return sandboxPath(f.SandboxDir, relative)
}

// ── file.write ───────────────────────────────────────────────────────

type FileWrite struct {
	SandboxDir string
}

func (f FileWrite) ID() string          { return "file.write" }
func (f FileWrite) Name() string        { return "Write File" }
func (f FileWrite) Description() string { return "Write content to a file in the sandbox" }

func (f FileWrite) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path relative to sandbox"},
			"content": {"type": "string", "description": "Content to write"},
			"mode": {"type": "string", "enum": ["overwrite", "append", "create_new"], "default": "overwrite"}
		},
		"required": ["path", "content"]
	}`)
}

type fileWriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode"`
}

func (f FileWrite) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p fileWriteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("file.write: invalid args: %w", err)
	}
	if p.Mode == "" {
		p.Mode = "overwrite"
	}
	full, err := sandboxPath(f.SandboxDir, p.Path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return &Result{Content: fmt.Sprintf("Error creating directory: %v", err)}, nil
	}
	switch p.Mode {
	case "create_new":
		if _, err := os.Stat(full); err == nil {
			return &Result{Content: "File already exists"}, nil
		}
		if err := os.WriteFile(full, []byte(p.Content), 0644); err != nil {
			return &Result{Content: fmt.Sprintf("Error writing: %v", err)}, nil
		}
	case "append":
		fh, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return &Result{Content: fmt.Sprintf("Error opening: %v", err)}, nil
		}
		defer fh.Close()
		if _, err := fh.WriteString(p.Content); err != nil {
			return &Result{Content: fmt.Sprintf("Error appending: %v", err)}, nil
		}
	default:
		if err := os.WriteFile(full, []byte(p.Content), 0644); err != nil {
			return &Result{Content: fmt.Sprintf("Error writing: %v", err)}, nil
		}
	}
	return &Result{Success: true, Content: "File written", MimeType: "text/plain"}, nil
}

func (f FileWrite) ValidateArgs(args json.RawMessage) error {
	var p fileWriteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Path == "" || p.Content == "" {
		return fmt.Errorf("path and content are required")
	}
	return nil
}

// ── file.list ────────────────────────────────────────────────────────

type FileList struct {
	SandboxDir string
}

func (f FileList) ID() string          { return "file.list" }
func (f FileList) Name() string        { return "List Files" }
func (f FileList) Description() string { return "List files in a sandbox directory" }

func (f FileList) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "default": "."}
		},
		"required": []
	}`)
}

type fileListArgs struct {
	Path string `json:"path"`
}

func (f FileList) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p fileListArgs
	json.Unmarshal(args, &p)
	if p.Path == "" {
		p.Path = "."
	}
	full, err := sandboxPath(f.SandboxDir, p.Path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading directory: %v", err)}, nil
	}
	var b strings.Builder
	for _, e := range entries {
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		dir := ""
		if e.IsDir() {
			dir = "/"
		}
		b.WriteString(fmt.Sprintf("- %s%s%s\n", e.Name(), dir, size))
	}
	if b.Len() == 0 {
		b.WriteString("(empty directory)")
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (f FileList) ValidateArgs(args json.RawMessage) error { return nil }

// ── file.delete ──────────────────────────────────────────────────────

type FileDelete struct {
	SandboxDir string
}

func (f FileDelete) ID() string          { return "file.delete" }
func (f FileDelete) Name() string        { return "Delete File" }
func (f FileDelete) Description() string { return "Delete a file (moves to trash by default)" }

func (f FileDelete) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path relative to sandbox"},
			"permanent": {"type": "boolean", "default": false}
		},
		"required": ["path"]
	}`)
}

type fileDeleteArgs struct {
	Path      string `json:"path"`
	Permanent bool   `json:"permanent"`
}

func (f FileDelete) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p fileDeleteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("file.delete: invalid args: %w", err)
	}
	full, err := sandboxPath(f.SandboxDir, p.Path)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	if p.Permanent {
		if err := os.RemoveAll(full); err != nil {
			return &Result{Content: fmt.Sprintf("Error deleting: %v", err)}, nil
		}
		return &Result{Success: true, Content: "Permanently deleted"}, nil
	}
	trashDir := filepath.Join(f.SandboxDir, ".trash")
	os.MkdirAll(trashDir, 0755)
	dest := filepath.Join(trashDir, filepath.Base(full))
	if err := os.Rename(full, dest); err != nil {
		return &Result{Content: fmt.Sprintf("Error moving to trash: %v", err)}, nil
	}
	return &Result{Success: true, Content: "Moved to trash"}, nil
}

func (f FileDelete) ValidateArgs(args json.RawMessage) error {
	var p fileDeleteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Path == "" {
		return fmt.Errorf("path is required")
	}
	return nil
}

// ── shell.run ────────────────────────────────────────────────────────

type ShellRun struct{}

var shellWhitelist = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"wc": true, "find": true, "grep": true, "ps": true,
	"df": true, "du": true, "uname": true, "whoami": true,
	"date": true, "echo": true, "pwd": true,
}

func (ShellRun) ID() string          { return "shell.run" }
func (ShellRun) Name() string        { return "Shell Command" }
func (ShellRun) Description() string { return "Run a whitelisted shell command" }

func (ShellRun) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Command to run"},
			"args": {"type": "array", "items": {"type": "string"}}
		},
		"required": ["command"]
	}`)
}

type shellRunArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func (ShellRun) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p shellRunArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("shell.run: invalid args: %w", err)
	}
	if !shellWhitelist[p.Command] {
		return &Result{Content: fmt.Sprintf("Command %q is not whitelisted", p.Command)}, nil
	}
	for _, a := range p.Args {
		if strings.ContainsAny(a, ";|&$()`") {
			return &Result{Content: "Chaining and subshells are not allowed"}, nil
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, p.Command, p.Args...)
	out, err := cmd.Output()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	return &Result{Success: true, Content: string(out)}, nil
}

func (ShellRun) ValidateArgs(args json.RawMessage) error {
	var p shellRunArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Command == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}
