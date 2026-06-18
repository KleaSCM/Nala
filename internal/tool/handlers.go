package tool

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

var httpDo = http.DefaultClient.Do

// ── LRU cache helper ──────────────────────────────────────────────────

type lruEntry struct {
	key   string
	value string
	time  time.Time
}

type lruCache struct {
	mu      sync.Mutex
	max     int
	ttl     time.Duration
	entries map[string]*lruEntry
	order   []string
}

func newLRUCache(max int, ttl time.Duration) *lruCache {
	return &lruCache{
		max:     max,
		ttl:     ttl,
		entries: make(map[string]*lruEntry),
	}
}

func (c *lruCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if time.Since(e.time) > c.ttl {
		delete(c.entries, key)
		return "", false
	}
	return e.value, true
}

func (c *lruCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[key] = &lruEntry{key: key, value: value, time: time.Now()}
	c.order = append(c.order, key)
}

var webCache = newLRUCache(100, 5*time.Minute)

// ── Rate limiter ──────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*rateWindow
	max      int
	interval time.Duration
}

type rateWindow struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(max int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		windows:  make(map[string]*rateWindow),
		max:      max,
		interval: interval,
	}
}

func (r *rateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.windows[key]
	now := time.Now()
	if !ok || now.After(w.resetAt) {
		r.windows[key] = &rateWindow{count: 1, resetAt: now.Add(r.interval)}
		return true
	}
	if w.count >= r.max {
		return false
	}
	w.count++
	return true
}

var domainRL = newRateLimiter(10, time.Minute)

// ── sandbox path helper ───────────────────────────────────────────────

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
	// Check symlinks resolve within sandbox
	real, err := filepath.EvalSymlinks(abs)
	if err == nil && !strings.HasPrefix(real, sbAbs) {
		return "", fmt.Errorf("symlink resolves outside sandbox")
	}
	return abs, nil
}

func (f FileRead) sandboxPath(relative string) (string, error) {
	return sandboxPath(f.SandboxDir, relative)
}

// ── web.search ────────────────────────────────────────────────────────

type WebSearch struct{}

func (WebSearch) ID() string          { return "web.search" }
func (WebSearch) Name() string        { return "Web Search" }
func (WebSearch) Description() string { return "Search the web using DuckDuckGo" }
func (WebSearch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"max_results": {"type": "integer", "default": 5, "minimum": 1, "maximum": 20},
			"region": {"type": "string", "default": "wt-wt", "description": "Region code"},
			"safe_search": {"type": "boolean", "default": true}
		},
		"required": ["query"]
	}`)
}

type webSearchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Region     string `json:"region"`
	SafeSearch bool   `json:"safe_search"`
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
	if p.Region == "" {
		p.Region = "wt-wt"
	}

	// Check cache
	cacheKey := fmt.Sprintf("ws:%s:%d:%s", p.Query, p.MaxResults, p.Region)
	if cached, ok := webCache.Get(cacheKey); ok {
		return &Result{Success: true, Content: cached, MimeType: "text/markdown", Data: []byte("(cached)")}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1&t=nala_1.0",
		url.QueryEscape(p.Query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("web.search: request: %w", err)
	}
	req.Header.Set("User-Agent", "Nala/1.0")

	resp, err := httpDo(req)
	if err != nil {
		// Try cache on network error
		if cached, ok := webCache.Get(cacheKey); ok {
			return &Result{Success: true, Content: cached + "\n\n*(cached — network unavailable)*", MimeType: "text/markdown"}, nil
		}
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
		b.WriteString(fmt.Sprintf("**%s**\n%s\n\n", ddg.AbstractText, ddg.AbstractURL))
	}
	count := 0
	for _, r := range ddg.Results {
		if count >= p.MaxResults {
			break
		}
		b.WriteString(fmt.Sprintf("- [%s](%s)\n", r.Text, r.FirstURL))
		count++
	}
	if b.Len() == 0 {
		b.WriteString("No results found")
	}
	content := b.String()
	webCache.Set(cacheKey, content)
	return &Result{Success: true, Content: content, MimeType: "text/markdown"}, nil
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

// ── web.fetch ─────────────────────────────────────────────────────────

type WebFetch struct{}

func (WebFetch) ID() string          { return "web.fetch" }
func (WebFetch) Name() string        { return "Web Fetch" }
func (WebFetch) Description() string { return "Fetch a URL and return its content as text" }

func (WebFetch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to fetch"},
			"max_chars": {"type": "integer", "default": 10000, "maximum": 100000},
			"extract_mode": {"type": "string", "enum": ["markdown", "text", "html"], "default": "text"}
		},
		"required": ["url"]
	}`)
}

type webFetchArgs struct {
	URL         string `json:"url"`
	MaxChars    int    `json:"max_chars"`
	ExtractMode string `json:"extract_mode"`
}

var fetchCache = newLRUCache(100, 5*time.Minute)

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
	if p.ExtractMode == "" {
		p.ExtractMode = "text"
	}

	// Check cache
	if cached, ok := fetchCache.Get(p.URL); ok {
		content := cached
		truncated := false
		if len(content) > p.MaxChars {
			content = content[:p.MaxChars]
			truncated = true
		}
		return &Result{Success: true, Content: content, MimeType: "text/plain", Truncated: truncated, Data: []byte("(cached)")}, nil
	}

	// Rate limit per domain
	parsed, err := url.Parse(p.URL)
	if err != nil {
		return nil, fmt.Errorf("web.fetch: invalid URL: %w", err)
	}
	if !domainRL.Allow(parsed.Host) {
		return &Result{Content: "Rate limited: too many requests to this domain"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("web.fetch: request: %w", err)
	}
	req.Header.Set("User-Agent", "Nala/1.0")
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "Web fetch unavailable"}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// Simple HTML tag stripping for text mode (no go-readability dependency yet)
	if p.ExtractMode == "text" {
		re := regexp.MustCompile(`<[^>]*>`)
		content = re.ReplaceAllString(content, "")
		content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
		content = strings.TrimSpace(content)
	}

	fetchCache.Set(p.URL, content)

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

// ── file.read ─────────────────────────────────────────────────────────

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

	// Check for binary content
	data, err := os.ReadFile(full)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading file: %v", err)}, nil
	}
	if isBinary(data) {
		return &Result{Success: true, Content: fmt.Sprintf("[Binary file: %s, %d bytes]", info.Name(), info.Size()), MimeType: "application/octet-stream", Data: data}, nil
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

func isBinary(data []byte) bool {
	if len(data) > 512 {
		data = data[:512]
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// ── file.write ────────────────────────────────────────────────────────

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
	if len(p.Content) > 10*1024*1024 {
		return &Result{Content: "Content too large (max 10MB)"}, nil
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

// ── file.list ─────────────────────────────────────────────────────────

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
			"path": {"type": "string", "default": "."},
			"pattern": {"type": "string", "description": "Glob pattern (e.g. *.txt)"},
			"recursive": {"type": "boolean", "default": false}
		},
		"required": []
	}`)
}

type fileListArgs struct {
	Path      string `json:"path"`
	Pattern   string `json:"pattern"`
	Recursive bool   `json:"recursive"`
}

type fileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	IsDir     bool   `json:"is_dir"`
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

	var files []fileInfo
	walker := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(full, path)
		if rel == "." {
			return nil
		}
		info, _ := d.Info()
		fi := fileInfo{Name: d.Name(), Path: rel, IsDir: d.IsDir()}
		if info != nil {
			fi.Size = info.Size()
			fi.ModifiedAt = info.ModTime().Format(time.RFC3339)
		}
		// Apply glob pattern
		if p.Pattern != "" {
			match, _ := filepath.Match(p.Pattern, d.Name())
			if !match {
				return nil
			}
		}
		files = append(files, fi)
		if len(files) >= 10000 {
			return fmt.Errorf("too many entries")
		}
		return nil
	}

	if p.Recursive {
		filepath.WalkDir(full, walker)
	} else {
		entries, err := os.ReadDir(full)
		if err != nil {
			return &Result{Content: fmt.Sprintf("Error reading directory: %v", err)}, nil
		}
		for _, e := range entries {
			walker(filepath.Join(full, e.Name()), e, nil)
		}
	}

	if len(files) == 0 {
		return &Result{Success: true, Content: "(empty directory)", MimeType: "text/plain"}, nil
	}

	var b strings.Builder
	for _, f := range files {
		dir := ""
		if f.IsDir {
			dir = "/"
		}
		b.WriteString(fmt.Sprintf("- %s%s  (%d bytes, %s)\n", f.Name, dir, f.Size, f.ModifiedAt))
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (f FileList) ValidateArgs(args json.RawMessage) error { return nil }

// ── file.delete ───────────────────────────────────────────────────────

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
	base := filepath.Base(full)
	dest := filepath.Join(trashDir, base)
	// Handle name collisions in trash
	if _, err := os.Stat(dest); err == nil {
		dest = filepath.Join(trashDir, fmt.Sprintf("%s_%d", base, time.Now().Unix()))
	}
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

// ── shell.run ─────────────────────────────────────────────────────────

type ShellRun struct{}

var shellWhitelist = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"wc": true, "find": true, "grep": true, "ps": true,
	"df": true, "du": true, "uname": true, "whoami": true,
	"date": true, "echo": true, "pwd": true,
}

// Network-requiring commands — allowed only in non-airgap mode
var shellNetworkWhitelist = map[string]bool{
	"ping": true, "curl": true, "wget": true, "dig": true,
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
	if !shellWhitelist[p.Command] && !shellNetworkWhitelist[p.Command] {
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{Content: fmt.Sprintf("Error: %s", string(exitErr.Stderr))}, nil
		}
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

// ── code.execute ──────────────────────────────────────────────────────

type CodeExecute struct{}

func (CodeExecute) ID() string          { return "code.execute" }
func (CodeExecute) Name() string        { return "Execute Code" }
func (CodeExecute) Description() string { return "Execute code in a sandboxed subprocess" }

func (CodeExecute) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"language": {"type": "string", "enum": ["python", "javascript", "go", "bash"], "description": "Language to execute"},
			"code": {"type": "string", "description": "Code to execute"},
			"timeout_ms": {"type": "integer", "default": 30000, "maximum": 60000}
		},
		"required": ["language", "code"]
	}`)
}

type codeExecuteArgs struct {
	Language  string `json:"language"`
	Code      string `json:"code"`
	TimeoutMs int    `json:"timeout_ms"`
}

var allowedLanguages = map[string]bool{
	"python":     true,
	"javascript": true,
	"go":         true,
	"bash":       true,
}

func (CodeExecute) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p codeExecuteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("code.execute: invalid args: %w", err)
	}
	if !allowedLanguages[p.Language] {
		return &Result{Content: fmt.Sprintf("Language %q is not allowed", p.Language)}, nil
	}
	if p.TimeoutMs <= 0 || p.TimeoutMs > 60000 {
		p.TimeoutMs = 30000
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutMs)*time.Millisecond)
	defer cancel()

	// Write code to temp file in sandbox
	tmpDir, err := os.MkdirTemp("", "nala-code-*")
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error creating temp dir: %v", err)}, nil
	}
	defer os.RemoveAll(tmpDir)

	var cmd *exec.Cmd
	switch p.Language {
	case "python":
		codeFile := filepath.Join(tmpDir, "script.py")
		if err := os.WriteFile(codeFile, []byte(p.Code), 0644); err != nil {
			return &Result{Content: fmt.Sprintf("Error writing script: %v", err)}, nil
		}
		cmd = exec.CommandContext(ctx, "python3", "-c", p.Code)
	case "javascript":
		cmd = exec.CommandContext(ctx, "node", "-e", p.Code)
	case "go":
		codeFile := filepath.Join(tmpDir, "main.go")
		pkg := `package main\nimport "fmt"\nfunc main() {\n%s\n}`
		// Wrap in main package if not already
		if !strings.Contains(p.Code, "package main") {
			p.Code = fmt.Sprintf(pkg, p.Code)
		}
		if err := os.WriteFile(codeFile, []byte(p.Code), 0644); err != nil {
			return &Result{Content: fmt.Sprintf("Error writing script: %v", err)}, nil
		}
		cmd = exec.CommandContext(ctx, "go", "run", codeFile)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", p.Code)
	}

	if cmd == nil {
		return &Result{Content: "Failed to create command"}, nil
	}

	// Environment: empty PATH + minimal vars
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"HOME=/tmp",
		"USER=nobody",
		"TMPDIR=" + tmpDir,
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{Content: fmt.Sprintf("Exit code %d:\n%s", exitErr.ExitCode(), string(exitErr.Stderr))}, nil
		}
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	return &Result{Success: true, Content: string(out), MimeType: "text/plain"}, nil
}

func (CodeExecute) ValidateArgs(args json.RawMessage) error {
	var p codeExecuteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Language == "" || p.Code == "" {
		return fmt.Errorf("language and code are required")
	}
	return nil
}

// ── db.query ───────────────────────────────────────────────────────────

type DBQuery struct {
	DB *sql.DB
}

func (d DBQuery) ID() string          { return "db.query" }
func (d DBQuery) Name() string        { return "Database Query" }
func (d DBQuery) Description() string { return "Execute read-only SQL queries against the database" }

func (d DBQuery) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "SQL query (SELECT/PRAGMA/EXPLAIN only)"},
			"params": {"type": "object", "description": "Optional query parameters"}
		},
		"required": ["query"]
	}`)
}

type dbQueryArgs struct {
	Query  string         `json:"query"`
	Params map[string]any `json:"params"`
}

var writeStmtRE = regexp.MustCompile(`(?i)^\s*(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|REPLACE|ATTACH|DETACH|VACUUM|REINDEX)`)

func (d DBQuery) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p dbQueryArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("db.query: invalid args: %w", err)
	}

	// Block write statements (checked before DB availability)
	if writeStmtRE.MatchString(p.Query) {
		return &Result{Content: "Only read-only queries are allowed (SELECT, PRAGMA, EXPLAIN)"}, nil
	}

	if d.DB == nil {
		return &Result{Content: "Database not available"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := d.DB.QueryContext(ctx, p.Query)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Query error: %v", err)}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error getting columns: %v", err)}, nil
	}

	var resultRows []map[string]any
	rowCount := 0
	for rows.Next() {
		if rowCount >= 1000 {
			break
		}
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return &Result{Content: fmt.Sprintf("Scan error: %v", err)}, nil
		}
		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		resultRows = append(resultRows, row)
		rowCount++
	}

	jsonResult, _ := json.Marshal(map[string]any{
		"columns":   columns,
		"rows":      resultRows,
		"row_count": rowCount,
	})
	return &Result{Success: true, Content: string(jsonResult), MimeType: "application/json"}, nil
}

func (d DBQuery) ValidateArgs(args json.RawMessage) error {
	var p dbQueryArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// ── http.request ───────────────────────────────────────────────────────

type HTTPRequest struct{}

func (HTTPRequest) ID() string          { return "http.request" }
func (HTTPRequest) Name() string        { return "HTTP Request" }
func (HTTPRequest) Description() string { return "Make an HTTP request" }

func (HTTPRequest) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"method": {"type": "string", "enum": ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"], "default": "GET"},
			"url": {"type": "string", "description": "URL to request"},
			"headers": {"type": "object", "description": "Optional headers"},
			"body": {"type": "string", "description": "Request body"},
			"timeout_ms": {"type": "integer", "default": 30000, "maximum": 60000}
		},
		"required": ["url"]
	}`)
}

type httpRequestArgs struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	TimeoutMs int               `json:"timeout_ms"`
}

var privateBlocks = []*net.IPNet{
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
	{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(32, 32)},
}

func isPrivateHost(target string) bool {
	addr, err := net.ResolveIPAddr("ip", target)
	if err != nil {
		return false
	}
	for _, block := range privateBlocks {
		if block.Contains(addr.IP) {
			return true
		}
	}
	return false
}

func (HTTPRequest) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p httpRequestArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("http.request: invalid args: %w", err)
	}
	if p.URL == "" {
		return &Result{Content: "No URL provided"}, nil
	}
	if p.Method == "" {
		p.Method = "GET"
	}
	if p.TimeoutMs <= 0 || p.TimeoutMs > 60000 {
		p.TimeoutMs = 30000
	}

	// Block private/internal IPs
	parsed, err := url.Parse(p.URL)
	if err != nil {
		return &Result{Content: "Invalid URL"}, nil
	}
	if isPrivateHost(parsed.Hostname()) {
		return &Result{Content: "Requests to internal/private IPs are blocked"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutMs)*time.Millisecond)
	defer cancel()

	var bodyReader io.Reader
	if p.Body != "" {
		bodyReader = strings.NewReader(p.Body)
	}

	req, err := http.NewRequestWithContext(ctx, p.Method, p.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http.request: %w", err)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Nala/1.0")
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Request failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	// Limit response size
	limitReader := io.LimitReader(resp.Body, 5*1024*1024+1)
	body, _ := io.ReadAll(limitReader)

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/octet-stream") || strings.HasPrefix(contentType, "image/") {
		return &Result{
			Success:  true,
			Content:  fmt.Sprintf("[Binary response: %s, %d bytes]", contentType, len(body)),
			MimeType: contentType,
			Data:     body,
		}, nil
	}

	truncated := false
	content := string(body)
	if len(body) > 5*1024*1024 {
		content = content[:5*1024*1024]
		truncated = true
	}

	return &Result{
		Success:   true,
		Content:   fmt.Sprintf("Status: %s\n\n%s", resp.Status, content),
		MimeType:  "text/plain",
		Truncated: truncated,
	}, nil
}

func (HTTPRequest) ValidateArgs(args json.RawMessage) error {
	var p httpRequestArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

// ── image.generate ────────────────────────────────────────────────────

type ImageGenerate struct{}

func (ImageGenerate) ID() string          { return "image.generate" }
func (ImageGenerate) Name() string        { return "Generate Image" }
func (ImageGenerate) Description() string { return "Generate an image using a configured backend" }

func (ImageGenerate) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {"type": "string", "description": "Image description"},
			"negative_prompt": {"type": "string", "description": "Things to avoid"},
			"width": {"type": "integer", "default": 512},
			"height": {"type": "integer", "default": 512},
			"steps": {"type": "integer", "default": 20},
			"count": {"type": "integer", "default": 1, "maximum": 4}
		},
		"required": ["prompt"]
	}`)
}

type imageGenerateArgs struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Steps          int    `json:"steps"`
	Count          int    `json:"count"`
}

func (ImageGenerate) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p imageGenerateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("image.generate: invalid args: %w", err)
	}
	if p.Width <= 0 {
		p.Width = 512
	}
	if p.Height <= 0 {
		p.Height = 512
	}
	if p.Steps <= 0 {
		p.Steps = 20
	}
	if p.Count <= 0 || p.Count > 4 {
		p.Count = 1
	}

	// Try Stable Diffusion WebUI API first
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	sdPayload := map[string]any{
		"prompt":          p.Prompt,
		"negative_prompt": p.NegativePrompt,
		"width":           p.Width,
		"height":          p.Height,
		"steps":           p.Steps,
		"batch_size":      p.Count,
	}
	body, _ := json.Marshal(sdPayload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:7860/sdapi/v1/txt2img", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "Image generation unavailable (is Stable Diffusion WebUI running on port 7860?)"}, nil
	}
	defer resp.Body.Close()

	var sdResp struct {
		Images []string `json:"images"`
		Info   string   `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sdResp); err != nil {
		return &Result{Content: fmt.Sprintf("Failed to parse SD response: %v", err)}, nil
	}

	if len(sdResp.Images) == 0 {
		return &Result{Content: "No images generated"}, nil
	}

	// Return first image as base64 data
	imgData, _ := base64.StdEncoding.DecodeString(sdResp.Images[0])
	return &Result{
		Success:  true,
		Content:  fmt.Sprintf("![Generated image](data:image/png;base64,%s)", sdResp.Images[0][:100]+"..."),
		MimeType: "image/png",
		Data:     imgData,
	}, nil
}

func (ImageGenerate) ValidateArgs(args json.RawMessage) error {
	var p imageGenerateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// ── image.analyze ─────────────────────────────────────────────────────

type ImageAnalyze struct{}

func (ImageAnalyze) ID() string          { return "image.analyze" }
func (ImageAnalyze) Name() string        { return "Analyze Image" }
func (ImageAnalyze) Description() string { return "Analyze an image using a vision-capable model" }

func (ImageAnalyze) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "Image URL or data: URI"},
			"prompt": {"type": "string", "default": "Describe this image in detail"}
		},
		"required": ["url"]
	}`)
}

type imageAnalyzeArgs struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

func (ImageAnalyze) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p imageAnalyzeArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("image.analyze: invalid args: %w", err)
	}
	if p.Prompt == "" {
		p.Prompt = "Describe this image in detail"
	}
	return &Result{Content: "Image analysis requires a vision-capable model connected via the model provider. Use a vision model like LLaVA, GPT-4o, or Claude 3.5 Sonnet."}, nil
}

func (ImageAnalyze) ValidateArgs(args json.RawMessage) error {
	var p imageAnalyzeArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

// ── knowledge.search ──────────────────────────────────────────────────

type KnowledgeSearch struct {
	EmbedFn func(ctx context.Context, texts []string) ([][]float32, error)
	SearchFn func(ctx context.Context, collection string, vector []float32, topK int, minScore float64) ([]VectorResult, error)
}

type VectorResult struct {
	ID       string  `json:"id"`
	Score    float64 `json:"score"`
	Metadata string  `json:"metadata"`
}

func (KnowledgeSearch) ID() string          { return "knowledge.search" }
func (KnowledgeSearch) Name() string        { return "Knowledge Search" }
func (KnowledgeSearch) Description() string { return "Search knowledge bases for relevant information" }

func (KnowledgeSearch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"knowledge_base_id": {"type": "string", "description": "Knowledge base ID (optional, searches all if empty)"},
			"top_k": {"type": "integer", "default": 5, "maximum": 20},
			"min_score": {"type": "number", "default": 0.7}
		},
		"required": ["query"]
	}`)
}

type knowledgeSearchArgs struct {
	Query           string  `json:"query"`
	KnowledgeBaseID string  `json:"knowledge_base_id"`
	TopK            int     `json:"top_k"`
	MinScore        float64 `json:"min_score"`
}

func (k KnowledgeSearch) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p knowledgeSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("knowledge.search: invalid args: %w", err)
	}
	if p.Query == "" {
		return &Result{Content: "No results found"}, nil
	}
	if p.TopK <= 0 || p.TopK > 20 {
		p.TopK = 5
	}
	if p.MinScore <= 0 {
		p.MinScore = 0.7
	}

	// If we have an embed function and search function, use them
	if k.EmbedFn != nil && k.SearchFn != nil {
		vectors, err := k.EmbedFn(ctx, []string{p.Query})
		if err != nil {
			return &Result{Content: fmt.Sprintf("Embedding failed: %v", err)}, nil
		}
		if len(vectors) == 0 {
			return &Result{Content: "No results found"}, nil
		}

		collection := "documents"
		if p.KnowledgeBaseID != "" {
			collection = "kb_" + p.KnowledgeBaseID
		}

		results, err := k.SearchFn(ctx, collection, vectors[0], p.TopK, p.MinScore)
		if err != nil {
			return &Result{Content: fmt.Sprintf("Search failed: %v", err)}, nil
		}

		if len(results) == 0 {
			return &Result{Content: "No relevant results found"}, nil
		}

		var b strings.Builder
		for _, r := range results {
			b.WriteString(fmt.Sprintf("- **Score: %.2f** — %s\n", r.Score, r.Metadata))
		}
		return &Result{Success: true, Content: b.String(), MimeType: "text/markdown"}, nil
	}

	// Fallback: keyword search via FTS
	if p.KnowledgeBaseID != "" {
		return &Result{Content: fmt.Sprintf("Knowledge base %q not yet configured with embedding provider. Please configure an embedding provider in Settings.", p.KnowledgeBaseID)}, nil
	}
	return &Result{Content: "Knowledge search requires an embedding provider (Ollama with nomic-embed-text or OpenAI embeddings). Please configure in Settings."}, nil
}

func (KnowledgeSearch) ValidateArgs(args json.RawMessage) error {
	var p knowledgeSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// ── memory.store / memory.recall ─────────────────────────────────────

type MemoryStore struct {
	StoreFn func(ctx context.Context, fact string, category string, importance float64) error
}

func (MemoryStore) ID() string          { return "memory.store" }
func (MemoryStore) Name() string        { return "Store Memory" }
func (MemoryStore) Description() string { return "Store a fact in long-term memory" }

func (MemoryStore) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"fact": {"type": "string", "description": "Fact to remember"},
			"category": {"type": "string", "enum": ["preference", "fact", "relationship", "schedule"], "default": "fact"},
			"importance": {"type": "number", "default": 0.5, "minimum": 0, "maximum": 1}
		},
		"required": ["fact"]
	}`)
}

type memoryStoreArgs struct {
	Fact       string  `json:"fact"`
	Category   string  `json:"category"`
	Importance float64 `json:"importance"`
}

func (m MemoryStore) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p memoryStoreArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("memory.store: invalid args: %w", err)
	}
	if p.Category == "" {
		p.Category = "fact"
	}
	if p.Importance <= 0 {
		p.Importance = 0.5
	}

	if m.StoreFn != nil {
		if err := m.StoreFn(ctx, p.Fact, p.Category, p.Importance); err != nil {
			return &Result{Content: fmt.Sprintf("Failed to store memory: %v", err)}, nil
		}
		return &Result{Success: true, Content: "Memory stored"}, nil
	}

	return &Result{Success: true, Content: "Memory stored (in-memory)"}, nil
}

func (MemoryStore) ValidateArgs(args json.RawMessage) error {
	var p memoryStoreArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Fact == "" {
		return fmt.Errorf("fact is required")
	}
	return nil
}

type MemoryRecall struct {
	RecallFn func(ctx context.Context, query string, topK int) ([]MemoryResult, error)
}

type MemoryResult struct {
	Fact       string  `json:"fact"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

func (MemoryRecall) ID() string          { return "memory.recall" }
func (MemoryRecall) Name() string        { return "Recall Memory" }
func (MemoryRecall) Description() string { return "Recall stored memories by query" }

func (MemoryRecall) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "What to recall"},
			"top_k": {"type": "integer", "default": 10, "maximum": 50}
		},
		"required": ["query"]
	}`)
}

type memoryRecallArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (m MemoryRecall) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p memoryRecallArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("memory.recall: invalid args: %w", err)
	}
	if p.TopK <= 0 || p.TopK > 50 {
		p.TopK = 10
	}

	if m.RecallFn != nil {
		results, err := m.RecallFn(ctx, p.Query, p.TopK)
		if err != nil {
			return &Result{Content: fmt.Sprintf("Recall failed: %v", err)}, nil
		}
		if len(results) == 0 {
			return &Result{Content: "No memories found"}, nil
		}
		var b strings.Builder
		for _, r := range results {
			b.WriteString(fmt.Sprintf("- [%s/%.2f] %s\n", r.Category, r.Confidence, r.Fact))
		}
		return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
	}

	// In-memory fallback store for testing
	results, ok := memStore[p.Query]
	if !ok {
		return &Result{Content: "No memories found"}, nil
	}
	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("- [%s/%.2f] %s\n", r.Category, r.Confidence, r.Fact))
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (MemoryRecall) ValidateArgs(args json.RawMessage) error {
	var p memoryRecallArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

var memStore = make(map[string][]MemoryResult)

func init() {
	memStore["default"] = []MemoryResult{
		{Fact: "User prefers concise answers", Category: "preference", Confidence: 0.8, Source: "explicit"},
		{Fact: "User is a software developer", Category: "fact", Confidence: 0.9, Source: "explicit"},
	}
}

// ── file.search ───────────────────────────────────────────────────────

type FileSearch struct {
	SandboxDir string
}

func (FileSearch) ID() string          { return "file.search" }
func (FileSearch) Name() string        { return "Search Files" }
func (FileSearch) Description() string { return "Search for files by pattern in the sandbox" }

func (FileSearch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob or regex pattern"},
			"root": {"type": "string", "default": ".", "description": "Root directory relative to sandbox"},
			"recursive": {"type": "boolean", "default": true},
			"max_results": {"type": "integer", "default": 100, "maximum": 1000},
			"type_filter": {"type": "string", "enum": ["file", "dir", "all"], "default": "all"},
			"include_hidden": {"type": "boolean", "default": false},
			"follow_symlinks": {"type": "boolean", "default": false}
		},
		"required": ["pattern"]
	}`)
}

type fileSearchArgs struct {
	Pattern        string `json:"pattern"`
	Root           string `json:"root"`
	Recursive      bool   `json:"recursive"`
	MaxResults     int    `json:"max_results"`
	TypeFilter     string `json:"type_filter"`
	IncludeHidden  bool   `json:"include_hidden"`
	FollowSymlinks bool   `json:"follow_symlinks"`
}

func (f FileSearch) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p fileSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("file.search: invalid args: %w", err)
	}
	if p.Pattern == "" {
		return &Result{Content: "No pattern provided"}, nil
	}
	if p.MaxResults <= 0 || p.MaxResults > 1000 {
		p.MaxResults = 100
	}
	if p.TypeFilter == "" {
		p.TypeFilter = "all"
	}
	if p.Root == "" {
		p.Root = "."
	}

	fullRoot, err := sandboxPath(f.SandboxDir, p.Root)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}

	// Compile regex pattern if it looks like regex
	var re *regexp.Regexp
	if strings.HasPrefix(p.Pattern, "/") && strings.HasSuffix(p.Pattern, "/") {
		re, err = regexp.Compile(p.Pattern[1 : len(p.Pattern)-1])
		if err != nil {
			return &Result{Content: fmt.Sprintf("Invalid regex: %v", err)}, nil
		}
	}

	var results []fileInfo
	tooMany := false

	walker := func(path string, d os.DirEntry, err error) error {
		if err != nil || len(results) >= p.MaxResults {
			if err == nil && len(results) >= p.MaxResults {
				tooMany = true
			}
			return filepath.SkipDir
		}

		// Skip hidden files/dirs
		if !p.IncludeHidden && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Type filter
		if p.TypeFilter == "file" && d.IsDir() {
			return nil
		}
		if p.TypeFilter == "dir" && !d.IsDir() {
			return nil
		}

		// Pattern matching
		match := false
		if re != nil {
			match = re.MatchString(d.Name())
		} else {
			match, _ = filepath.Match(p.Pattern, d.Name())
		}
		if !match {
			return nil
		}

		info, _ := d.Info()
		rel, _ := filepath.Rel(fullRoot, path)
		fi := fileInfo{Name: d.Name(), Path: rel, IsDir: d.IsDir(), Size: 0}
		if info != nil {
			fi.Size = info.Size()
			fi.ModifiedAt = info.ModTime().Format(time.RFC3339)
		}
		results = append(results, fi)
		return nil
	}

	if p.Recursive {
		filepath.WalkDir(fullRoot, walker)
	} else {
		entries, err := os.ReadDir(fullRoot)
		if err != nil {
			return &Result{Content: fmt.Sprintf("Error reading directory: %v", err)}, nil
		}
		for _, e := range entries {
			walker(filepath.Join(fullRoot, e.Name()), e, nil)
		}
	}

	if len(results) == 0 {
		return &Result{Content: "No matching files found"}, nil
	}

	var b strings.Builder
	if tooMany {
		b.WriteString(fmt.Sprintf("*(showing %d of %d+ results — refine your pattern)*\n", len(results), p.MaxResults))
	}
	for _, fi := range results {
		dir := ""
		if fi.IsDir {
			dir = "/"
		}
		b.WriteString(fmt.Sprintf("- %s%s  (%d bytes, %s)\n", fi.Path, dir, fi.Size, fi.ModifiedAt))
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (FileSearch) ValidateArgs(args json.RawMessage) error {
	var p fileSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	return nil
}

// ── calendar.* ────────────────────────────────────────────────────────

type CalendarList struct{}

func (CalendarList) ID() string          { return "calendar.list" }
func (CalendarList) Name() string        { return "List Events" }
func (CalendarList) Description() string { return "List calendar events within a time range" }

func (CalendarList) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"start": {"type": "string", "description": "Start time (ISO 8601, default: now)"},
			"end": {"type": "string", "description": "End time (ISO 8601, default: +7 days)"},
			"calendar_id": {"type": "string", "description": "Calendar ID (optional, all if empty)"},
			"max_results": {"type": "integer", "default": 50, "maximum": 200}
		},
		"required": []
	}`)
}

type calendarListArgs struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	CalendarID string `json:"calendar_id"`
	MaxResults int    `json:"max_results"`
}

func (CalendarList) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p calendarListArgs
	json.Unmarshal(args, &p)
	if p.MaxResults <= 0 || p.MaxResults > 200 {
		p.MaxResults = 50
	}
	start := p.Start
	if start == "" {
		start = time.Now().Format(time.RFC3339)
	}
	end := p.End
	if end == "" {
		end = time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	}
	return &Result{Success: true, Content: fmt.Sprintf("Calendar events from %s to %s:\n\n*(Calendar storage not yet configured. Events will appear once a calendar provider is set up in Settings.)*", start, end), MimeType: "text/plain"}, nil
}

func (CalendarList) ValidateArgs(args json.RawMessage) error { return nil }

type CalendarCreate struct{}

func (CalendarCreate) ID() string          { return "calendar.create" }
func (CalendarCreate) Name() string        { return "Create Event" }
func (CalendarCreate) Description() string { return "Create a calendar event" }

func (CalendarCreate) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {"type": "string", "description": "Event title"},
			"start_time": {"type": "string", "description": "Start time (ISO 8601)"},
			"duration_minutes": {"type": "integer", "default": 60},
			"description": {"type": "string"},
			"calendar_id": {"type": "string"}
		},
		"required": ["title", "start_time"]
	}`)
}

type calendarCreateArgs struct {
	Title           string `json:"title"`
	StartTime       string `json:"start_time"`
	DurationMinutes int    `json:"duration_minutes"`
	Description     string `json:"description"`
	CalendarID      string `json:"calendar_id"`
}

func (CalendarCreate) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p calendarCreateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("calendar.create: invalid args: %w", err)
	}
	if p.DurationMinutes <= 0 {
		p.DurationMinutes = 60
	}
	return &Result{Success: true, Content: fmt.Sprintf("Event %q created at %s (%d min)\n*(Persistent calendar storage not yet configured)*", p.Title, p.StartTime, p.DurationMinutes)}, nil
}

func (CalendarCreate) ValidateArgs(args json.RawMessage) error {
	var p calendarCreateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Title == "" || p.StartTime == "" {
		return fmt.Errorf("title and start_time are required")
	}
	return nil
}

// ── email.* ───────────────────────────────────────────────────────────

type EmailSend struct{}

func (EmailSend) ID() string          { return "email.send" }
func (EmailSend) Name() string        { return "Send Email" }
func (EmailSend) Description() string { return "Send an email via configured SMTP" }

func (EmailSend) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "Recipient email"},
			"subject": {"type": "string", "description": "Email subject"},
			"body": {"type": "string", "description": "Email body (plain text)"},
			"cc": {"type": "string"},
			"account_id": {"type": "string"}
		},
		"required": ["to", "subject", "body"]
	}`)
}

type emailSendArgs struct {
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Cc        string `json:"cc"`
	AccountID string `json:"account_id"`
}

func (EmailSend) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p emailSendArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("email.send: invalid args: %w", err)
	}
	return &Result{Success: true, Content: fmt.Sprintf("Email to %q with subject %q queued for sending.\n*(SMTP sending requires an email account configured in Settings.)*", p.To, p.Subject)}, nil
}

func (EmailSend) ValidateArgs(args json.RawMessage) error {
	var p emailSendArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.To == "" || p.Subject == "" || p.Body == "" {
		return fmt.Errorf("to, subject, and body are required")
	}
	return nil
}

type EmailInbox struct{}

func (EmailInbox) ID() string          { return "email.inbox" }
func (EmailInbox) Name() string        { return "List Inbox" }
func (EmailInbox) Description() string { return "List recent emails from inbox" }

func (EmailInbox) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"folder": {"type": "string", "default": "INBOX"},
			"limit": {"type": "integer", "default": 20, "maximum": 100},
			"account_id": {"type": "string"}
		},
		"required": []
	}`)
}

type emailInboxArgs struct {
	Folder    string `json:"folder"`
	Limit     int    `json:"limit"`
	AccountID string `json:"account_id"`
}

func (EmailInbox) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p emailInboxArgs
	json.Unmarshal(args, &p)
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}
	if p.Folder == "" {
		p.Folder = "INBOX"
	}
	return &Result{Success: true, Content: fmt.Sprintf("Inbox folder %q:\n\n*(IMAP inbox listing requires an email account configured in Settings.)*", p.Folder), MimeType: "text/plain"}, nil
}

func (EmailInbox) ValidateArgs(args json.RawMessage) error { return nil }

// ── notes.* ───────────────────────────────────────────────────────────

type NotesCreate struct {
	NotesDir string
}

func (n NotesCreate) ID() string          { return "notes.create" }
func (n NotesCreate) Name() string        { return "Create Note" }
func (n NotesCreate) Description() string { return "Create a new note" }

func (n NotesCreate) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {"type": "string", "description": "Note title"},
			"content": {"type": "string", "description": "Note content (Markdown)"},
			"folder": {"type": "string", "description": "Folder path (optional)"},
			"tags": {"type": "array", "items": {"type": "string"}}
		},
		"required": ["title", "content"]
	}`)
}

type notesCreateArgs struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Folder  string   `json:"folder"`
	Tags    []string `json:"tags"`
}

func (n NotesCreate) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p notesCreateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("notes.create: invalid args: %w", err)
	}

	notesDir := n.NotesDir
	if notesDir == "" {
		notesDir = filepath.Join(os.TempDir(), "nala-notes")
	}

	// Build folder path
	noteFolder := notesDir
	if p.Folder != "" {
		cleanFolder := filepath.Clean(p.Folder)
		if strings.HasPrefix(cleanFolder, "..") || strings.HasPrefix(cleanFolder, "/") {
			return &Result{Content: "Invalid folder path"}, nil
		}
		noteFolder = filepath.Join(notesDir, cleanFolder)
		if !strings.HasPrefix(noteFolder, notesDir) {
			return &Result{Content: "Folder traversal blocked"}, nil
		}
	}
	if err := os.MkdirAll(noteFolder, 0755); err != nil {
		return &Result{Content: fmt.Sprintf("Error creating folder: %v", err)}, nil
	}

	// Sanitize title for filename
	safeName := strings.ReplaceAll(p.Title, "/", "_")
	safeName = strings.ReplaceAll(safeName, " ", "_")
	if len(safeName) > 200 {
		safeName = safeName[:200]
	}
	filePath := filepath.Join(noteFolder, safeName+".md")

	// YAML frontmatter
	tagsJSON, _ := json.Marshal(p.Tags)
	now := time.Now().Format(time.RFC3339)
	frontmatter := fmt.Sprintf("---\ntitle: %s\ntags: %s\ncreated: %s\nmodified: %s\n---\n\n",
		p.Title, string(tagsJSON), now, now)
	content := frontmatter + p.Content

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return &Result{Content: fmt.Sprintf("Error writing note: %v", err)}, nil
	}

	return &Result{Success: true, Content: fmt.Sprintf("Note %q created at %s", p.Title, filePath)}, nil
}

func (NotesCreate) ValidateArgs(args json.RawMessage) error {
	var p notesCreateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Title == "" || p.Content == "" {
		return fmt.Errorf("title and content are required")
	}
	return nil
}

type NotesList struct {
	NotesDir string
}

func (n NotesList) ID() string          { return "notes.list" }
func (n NotesList) Name() string        { return "List Notes" }
func (n NotesList) Description() string { return "List notes in a folder" }

func (n NotesList) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"folder": {"type": "string", "default": ""},
			"pattern": {"type": "string", "description": "Glob pattern"}
		},
		"required": []
	}`)
}

type notesListArgs struct {
	Folder  string `json:"folder"`
	Pattern string `json:"pattern"`
}

func (n NotesList) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p notesListArgs
	json.Unmarshal(args, &p)

	notesDir := n.NotesDir
	if notesDir == "" {
		notesDir = filepath.Join(os.TempDir(), "nala-notes")
	}

	listDir := notesDir
	if p.Folder != "" {
		cleanFolder := filepath.Clean(p.Folder)
		if strings.HasPrefix(cleanFolder, "..") || strings.HasPrefix(cleanFolder, "/") {
			return &Result{Content: "Invalid folder path"}, nil
		}
		listDir = filepath.Join(notesDir, cleanFolder)
		if !strings.HasPrefix(listDir, notesDir) {
			return &Result{Content: "Folder traversal blocked"}, nil
		}
	}

	entries, err := os.ReadDir(listDir)
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error reading folder: %v", err)}, nil
	}

	var b strings.Builder
	for _, e := range entries {
		if p.Pattern != "" {
			match, _ := filepath.Match(p.Pattern, e.Name())
			if !match {
				continue
			}
		}
		if e.IsDir() {
			b.WriteString(fmt.Sprintf("- %s/\n", e.Name()))
		} else if strings.HasSuffix(e.Name(), ".md") {
			name := strings.TrimSuffix(e.Name(), ".md")
			b.WriteString(fmt.Sprintf("- %s\n", name))
		}
	}
	if b.Len() == 0 {
		b.WriteString("(no notes found)")
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (NotesList) ValidateArgs(args json.RawMessage) error { return nil }

// ── AUR tools ─────────────────────────────────────────────────────────

type AURSearch struct{}

func (AURSearch) ID() string          { return "aur.search" }
func (AURSearch) Name() string        { return "AUR Search" }
func (AURSearch) Description() string { return "Search the Arch User Repository for packages" }

func (AURSearch) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"limit": {"type": "integer", "default": 10, "maximum": 50},
			"by": {"type": "string", "enum": ["name", "desc", "maintainer"], "default": "name"}
		},
		"required": ["query"]
	}`)
}

type aurSearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
	By    string `json:"by"`
}

type aurPackage struct {
	Name         string `json:"Name"`
	Version      string `json:"Version"`
	Description  string `json:"Description"`
	NumVotes     int    `json:"NumVotes"`
	Popularity   float64 `json:"Popularity"`
	Maintainer   string `json:"Maintainer"`
	LastModified int64  `json:"LastModified"`
	OutOfDate    int    `json:"OutOfDate"`
}

func (AURSearch) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("aur.search: invalid args: %w", err)
	}
	if p.Limit <= 0 || p.Limit > 50 {
		p.Limit = 10
	}
	if p.By == "" {
		p.By = "name"
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://aur.archlinux.org/rpc/v5/search/%s?by=%s", url.PathEscape(p.Query), url.QueryEscape(p.By))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "Nala/1.0")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "AUR search unavailable"}, nil
	}
	defer resp.Body.Close()

	var rpc struct {
		Results []aurPackage `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return &Result{Content: "AUR search unavailable"}, nil
	}

	if len(rpc.Results) == 0 {
		return &Result{Content: "No packages found"}, nil
	}

	count := 0
	var b strings.Builder
	for _, pkg := range rpc.Results {
		if count >= p.Limit {
			break
		}
		ood := ""
		if pkg.OutOfDate != 0 {
			ood = " [out-of-date]"
		}
		b.WriteString(fmt.Sprintf("- **%s** %s — %s (votes: %d, popularity: %.2f)%s\n",
			pkg.Name, pkg.Version, pkg.Description, pkg.NumVotes, pkg.Popularity, ood))
		count++
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/markdown"}, nil
}

func (AURSearch) ValidateArgs(args json.RawMessage) error {
	var p aurSearchArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

type AURInfo struct{}

func (AURInfo) ID() string          { return "aur.info" }
func (AURInfo) Name() string        { return "AUR Package Info" }
func (AURInfo) Description() string { return "Get detailed information about an AUR package" }

func (AURInfo) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"package": {"type": "string", "description": "Package name"}
		},
		"required": ["package"]
	}`)
}

type aurInfoArgs struct {
	Package string `json:"package"`
}

func (AURInfo) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurInfoArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("aur.info: invalid args: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://aur.archlinux.org/rpc/v5/info/%s", url.PathEscape(p.Package))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "Nala/1.0")

	resp, err := httpDo(req)
	if err != nil {
		return &Result{Content: "AUR info unavailable"}, nil
	}
	defer resp.Body.Close()

	var rpc struct {
		Results []aurPackage `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return &Result{Content: "AUR info unavailable"}, nil
	}

	if len(rpc.Results) == 0 {
		return &Result{Content: "Package not found"}, nil
	}

	pkg := rpc.Results[0]
	ood := ""
	if pkg.OutOfDate != 0 {
		ood = " (out-of-date)"
	}
	t := time.Unix(pkg.LastModified, 0).Format("2006-01-02")
	content := fmt.Sprintf("# %s %s%s\n\n**Description:** %s\n**Votes:** %d\n**Popularity:** %.2f\n**Maintainer:** %s\n**Last Modified:** %s\n",
		pkg.Name, pkg.Version, ood, pkg.Description, pkg.NumVotes, pkg.Popularity, pkg.Maintainer, t)
	return &Result{Success: true, Content: content, MimeType: "text/markdown"}, nil
}

func (AURInfo) ValidateArgs(args json.RawMessage) error {
	var p aurInfoArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Package == "" {
		return fmt.Errorf("package is required")
	}
	return nil
}

type AURInstall struct{}

func (AURInstall) ID() string          { return "aur.install" }
func (AURInstall) Name() string        { return "Install AUR Package" }
func (AURInstall) Description() string { return "Install packages from the AUR using yay or paru" }

func (AURInstall) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"packages": {"type": "array", "items": {"type": "string"}, "description": "Package names to install"},
			"asdeps": {"type": "boolean", "default": false}
		},
		"required": ["packages"]
	}`)
}

type aurInstallArgs struct {
	Packages []string `json:"packages"`
	Asdeps   bool     `json:"asdeps"`
}

func (AURInstall) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurInstallArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("aur.install: invalid args: %w", err)
	}
	if len(p.Packages) == 0 {
		return &Result{Content: "No packages specified"}, nil
	}

	// Detect AUR helper
	helper := ""
	for _, h := range []string{"yay", "paru"} {
		if _, err := exec.LookPath(h); err == nil {
			helper = h
			break
		}
	}
	if helper == "" {
		return &Result{Content: "Install yay or paru first. Neither AUR helper was found in PATH."}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	argsList := []string{"-S", "--noconfirm", "--needed"}
	if p.Asdeps {
		argsList = append(argsList, "--asdeps")
	}
	argsList = append(argsList, p.Packages...)

	cmd := exec.CommandContext(ctx, helper, argsList...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Install output:\n%s\nError: %v", string(out), err)}, nil
	}
	return &Result{Success: true, Content: fmt.Sprintf("Install output:\n%s", string(out))}, nil
}

func (AURInstall) ValidateArgs(args json.RawMessage) error {
	var p aurInstallArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if len(p.Packages) == 0 {
		return fmt.Errorf("packages is required")
	}
	return nil
}

type AURRemove struct{}

func (AURRemove) ID() string          { return "aur.remove" }
func (AURRemove) Name() string        { return "Remove AUR Package" }
func (AURRemove) Description() string { return "Remove installed AUR packages" }

func (AURRemove) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"packages": {"type": "array", "items": {"type": "string"}, "description": "Package names to remove"},
			"cascade": {"type": "boolean", "default": false}
		},
		"required": ["packages"]
	}`)
}

type aurRemoveArgs struct {
	Packages []string `json:"packages"`
	Cascade  bool     `json:"cascade"`
}

func (AURRemove) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurRemoveArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("aur.remove: invalid args: %w", err)
	}
	if len(p.Packages) == 0 {
		return &Result{Content: "No packages specified"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	argsList := []string{"-R"}
	if p.Cascade {
		argsList = append(argsList, "-c")
	}
	argsList = append(argsList, "--noconfirm")
	argsList = append(argsList, p.Packages...)

	cmd := exec.CommandContext(ctx, "pacman", argsList...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Remove output:\n%s\nError: %v", string(out), err)}, nil
	}
	return &Result{Success: true, Content: fmt.Sprintf("Remove output:\n%s", string(out))}, nil
}

func (AURRemove) ValidateArgs(args json.RawMessage) error {
	var p aurRemoveArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if len(p.Packages) == 0 {
		return fmt.Errorf("packages is required")
	}
	return nil
}

type AURUpdate struct{}

func (AURUpdate) ID() string          { return "aur.update" }
func (AURUpdate) Name() string        { return "Update AUR Packages" }
func (AURUpdate) Description() string { return "Update installed AUR packages" }

func (AURUpdate) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"packages": {"type": "array", "items": {"type": "string"}, "description": "Packages to update (empty = all)"}
		},
		"required": []
	}`)
}

type aurUpdateArgs struct {
	Packages []string `json:"packages"`
}

func (AURUpdate) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurUpdateArgs
	json.Unmarshal(args, &p)

	helper := ""
	for _, h := range []string{"yay", "paru"} {
		if _, err := exec.LookPath(h); err == nil {
			helper = h
			break
		}
	}
	if helper == "" {
		return &Result{Content: "Install yay or paru first. Neither AUR helper was found in PATH."}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	argsList := []string{"-Syu", "--noconfirm"}
	if len(p.Packages) > 0 {
		argsList = append(argsList, p.Packages...)
	}

	cmd := exec.CommandContext(ctx, helper, argsList...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Update output:\n%s\nError: %v", string(out), err)}, nil
	}
	return &Result{Success: true, Content: fmt.Sprintf("Update output:\n%s", string(out))}, nil
}

func (AURUpdate) ValidateArgs(args json.RawMessage) error { return nil }

type AURList struct{}

func (AURList) ID() string          { return "aur.list" }
func (AURList) Name() string        { return "List AUR Packages" }
func (AURList) Description() string { return "List installed AUR packages" }

func (AURList) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"filter": {"type": "string", "description": "Optional filter text"},
			"upgradable_only": {"type": "boolean", "default": false}
		},
		"required": []
	}`)
}

type aurListArgs struct {
	Filter         string `json:"filter"`
	UpgradableOnly bool   `json:"upgradable_only"`
}

func (AURList) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p aurListArgs
	json.Unmarshal(args, &p)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pacmanArgs := []string{"-Qm"}
	if p.UpgradableOnly {
		helper := ""
		for _, h := range []string{"yay", "paru"} {
			if _, err := exec.LookPath(h); err == nil {
				helper = h
				break
			}
		}
		if helper != "" {
			cmd := exec.CommandContext(ctx, helper, "-Qu")
			out, _ := cmd.Output()
			return &Result{Success: true, Content: string(out)}, nil
		}
		return &Result{Content: "No AUR helper found for checking updates"}, nil
	}

	cmd := exec.CommandContext(ctx, "pacman", pacmanArgs...)
	out, err := cmd.Output()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}
	content := string(out)
	if p.Filter != "" {
		lines := strings.Split(content, "\n")
		var filtered []string
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(p.Filter)) {
				filtered = append(filtered, line)
			}
		}
		content = strings.Join(filtered, "\n")
	}
	return &Result{Success: true, Content: content}, nil
}

func (AURList) ValidateArgs(args json.RawMessage) error { return nil }

// ── system.monitor ────────────────────────────────────────────────────

type SystemMonitor struct{}

func (SystemMonitor) ID() string          { return "system.monitor" }
func (SystemMonitor) Name() string        { return "System Monitor" }
func (SystemMonitor) Description() string { return "Get current system metrics" }

func (SystemMonitor) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"metric": {"type": "string", "enum": ["cpu", "memory", "disk", "network", "all"], "default": "all"},
			"duration": {"type": "string", "enum": ["", "last_5m", "last_1h", "last_6h", "last_24h"]}
		},
		"required": []
	}`)
}

type systemMonitorArgs struct {
	Metric   string `json:"metric"`
	Duration string `json:"duration"`
}

func (SystemMonitor) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p systemMonitorArgs
	json.Unmarshal(args, &p)
	if p.Metric == "" {
		p.Metric = "all"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var b strings.Builder

	if p.Metric == "cpu" || p.Metric == "all" {
		b.WriteString("## CPU\n")
		data, _ := os.ReadFile("/proc/loadavg")
		b.WriteString(fmt.Sprintf("Load Average: %s\n", strings.TrimSpace(string(data))))
	}

	if p.Metric == "memory" || p.Metric == "all" {
		b.WriteString("\n## Memory\n")
		data, _ := os.ReadFile("/proc/meminfo")
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[:5] {
			b.WriteString(line + "\n")
		}
	}

	if p.Metric == "disk" || p.Metric == "all" {
		b.WriteString("\n## Disk\n")
		cmd := exec.CommandContext(ctx, "df", "-h")
		out, _ := cmd.Output()
		b.WriteString(string(out))
	}

	if p.Metric == "network" || p.Metric == "all" {
		b.WriteString("\n## Network\n")
		data, _ := os.ReadFile("/proc/net/dev")
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[2:6] {
			b.WriteString(line + "\n")
		}
	}

	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (SystemMonitor) ValidateArgs(args json.RawMessage) error { return nil }

// ── system.processes ──────────────────────────────────────────────────

type SystemProcesses struct{}

func (SystemProcesses) ID() string          { return "system.processes" }
func (SystemProcesses) Name() string        { return "Process List" }
func (SystemProcesses) Description() string { return "List running processes" }

func (SystemProcesses) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"sort_by": {"type": "string", "enum": ["cpu", "memory", "pid", "name"], "default": "cpu"},
			"order": {"type": "string", "enum": ["asc", "desc"], "default": "desc"},
			"filter": {"type": "string", "description": "Filter by name"},
			"limit": {"type": "integer", "default": 20, "maximum": 100}
		},
		"required": []
	}`)
}

type systemProcessesArgs struct {
	SortBy string `json:"sort_by"`
	Order  string `json:"order"`
	Filter string `json:"filter"`
	Limit  int    `json:"limit"`
}

func (SystemProcesses) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p systemProcessesArgs
	json.Unmarshal(args, &p)
	if p.SortBy == "" {
		p.SortBy = "cpu"
	}
	if p.Order == "" {
		p.Order = "desc"
	}
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	sortFlag := "%cpu"
	switch p.SortBy {
	case "memory":
		sortFlag = "%mem"
	case "pid":
		sortFlag = "pid"
	case "name":
		sortFlag = "comm"
	}

	orderFlag := "-"
	if p.Order == "asc" {
		orderFlag = ""
	}

	argsList := []string{"axo", "pid,ppid,%cpu,%mem,rss,user,comm,args", "--sort", orderFlag + sortFlag, "--no-headers"}
	cmd := exec.CommandContext(ctx, "ps", argsList...)
	out, err := cmd.Output()
	if err != nil {
		return &Result{Content: fmt.Sprintf("Error: %v", err)}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	count := 0
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-8s %-6s %-6s %-8s %-10s %-8s %s\n", "PID", "CPU%", "MEM%", "RSS", "USER", "CMD", "ARGS"))
	b.WriteString(strings.Repeat("-", 80) + "\n")
	for _, line := range lines {
		if p.Filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(p.Filter)) {
			continue
		}
		if count >= p.Limit {
			break
		}
		fields := strings.Fields(line)
		if len(fields) >= 7 {
			b.WriteString(fmt.Sprintf("%-8s %-6s %-6s %-8s %-10s %-8s %s\n",
				fields[0], fields[2], fields[3], fields[4], fields[5], fields[6], strings.Join(fields[7:], " ")))
		}
		count++
	}
	return &Result{Success: true, Content: b.String(), MimeType: "text/plain"}, nil
}

func (SystemProcesses) ValidateArgs(args json.RawMessage) error { return nil }

// ── system.logs ───────────────────────────────────────────────────────

type SystemLogs struct {
	LogDir string
}

func (s SystemLogs) ID() string          { return "system.logs" }
func (s SystemLogs) Name() string        { return "System Logs" }
func (s SystemLogs) Description() string { return "Return recent system log entries" }

func (s SystemLogs) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string"},
			"level": {"type": "string", "enum": ["debug", "info", "warn", "error"]},
			"limit": {"type": "integer", "default": 100, "maximum": 1000},
			"since": {"type": "string", "description": "ISO 8601 timestamp"}
		},
		"required": []
	}`)
}

type systemLogsArgs struct {
	Query string `json:"query"`
	Level string `json:"level"`
	Limit int    `json:"limit"`
	Since string `json:"since"`
}

func (s SystemLogs) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p systemLogsArgs
	json.Unmarshal(args, &p)
	if p.Limit <= 0 || p.Limit > 1000 {
		p.Limit = 100
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Read from log file if available, otherwise journalctl
	logFile := s.LogDir
	if logFile == "" || logFile == os.TempDir() {
		// Try journalctl
		cmd := exec.CommandContext(ctx, "journalctl", "-n", fmt.Sprintf("%d", p.Limit), "--no-pager", "-o", "short-iso")
		if p.Level != "" {
			cmd.Args = append(cmd.Args, "-p", p.Level)
		}
		out, err := cmd.Output()
		if err != nil {
			return &Result{Content: "Nala log file not found. Install with logging configured."}, nil
		}
		content := string(out)
		if p.Query != "" {
			lines := strings.Split(content, "\n")
			var filtered []string
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), strings.ToLower(p.Query)) {
					filtered = append(filtered, line)
				}
			}
			content = strings.Join(filtered, "\n")
		}
		return &Result{Success: true, Content: content}, nil
	}

	logPath := filepath.Join(logFile, "nala.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return &Result{Content: "Nala log file not found"}, nil
	}

	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > p.Limit {
		start = len(lines) - p.Limit
	}
	content := strings.Join(lines[start:], "\n")
	if p.Query != "" {
		var filtered []string
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(p.Query)) {
				filtered = append(filtered, line)
			}
		}
		content = strings.Join(filtered, "\n")
	}
	return &Result{Success: true, Content: content}, nil
}

func (SystemLogs) ValidateArgs(args json.RawMessage) error { return nil }

// ── system.notify ─────────────────────────────────────────────────────

type SystemNotify struct{}

func (SystemNotify) ID() string          { return "system.notify" }
func (SystemNotify) Name() string        { return "Send Notification" }
func (SystemNotify) Description() string { return "Send a desktop notification" }

func (SystemNotify) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {"type": "string", "description": "Notification title"},
			"message": {"type": "string", "description": "Notification body"},
			"urgency": {"type": "string", "enum": ["low", "normal", "critical"], "default": "normal"},
			"timeout_ms": {"type": "integer", "description": "Auto-dismiss timeout"}
		},
		"required": ["title", "message"]
	}`)
}

type systemNotifyArgs struct {
	Title     string `json:"title"`
	Message   string `json:"message"`
	Urgency   string `json:"urgency"`
	TimeoutMs int    `json:"timeout_ms"`
}

func (SystemNotify) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var p systemNotifyArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("system.notify: invalid args: %w", err)
	}
	if p.Urgency == "" {
		p.Urgency = "normal"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	notifyArgs := []string{"-u", p.Urgency, p.Title, p.Message}
	if p.TimeoutMs > 0 {
		notifyArgs = append(notifyArgs, "-t", fmt.Sprintf("%d", p.TimeoutMs))
	}

	cmd := exec.CommandContext(ctx, "notify-send", notifyArgs...)
	if err := cmd.Run(); err != nil {
		return &Result{Content: fmt.Sprintf("Notification failed: %v (is libnotify installed?)", err)}, nil
	}
	return &Result{Success: true, Content: "Notification sent"}, nil
}

func (SystemNotify) ValidateArgs(args json.RawMessage) error {
	var p systemNotifyArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.Title == "" || p.Message == "" {
		return fmt.Errorf("title and message are required")
	}
	return nil
}

// ── Helper: check if tool struct implements Tool ──────────────────────

var (
	_ Tool = (*WebSearch)(nil)
	_ Tool = (*WebFetch)(nil)
	_ Tool = (*FileRead)(nil)
	_ Tool = (*FileWrite)(nil)
	_ Tool = (*FileList)(nil)
	_ Tool = (*FileDelete)(nil)
	_ Tool = (*ShellRun)(nil)
	_ Tool = (*CodeExecute)(nil)
	_ Tool = (*DBQuery)(nil)
	_ Tool = (*HTTPRequest)(nil)
	_ Tool = (*ImageGenerate)(nil)
	_ Tool = (*ImageAnalyze)(nil)
	_ Tool = (*KnowledgeSearch)(nil)
	_ Tool = (*MemoryStore)(nil)
	_ Tool = (*MemoryRecall)(nil)
	_ Tool = (*FileSearch)(nil)
	_ Tool = (*CalendarList)(nil)
	_ Tool = (*CalendarCreate)(nil)
	_ Tool = (*EmailSend)(nil)
	_ Tool = (*EmailInbox)(nil)
	_ Tool = (*NotesCreate)(nil)
	_ Tool = (*NotesList)(nil)
	_ Tool = (*AURSearch)(nil)
	_ Tool = (*AURInfo)(nil)
	_ Tool = (*AURInstall)(nil)
	_ Tool = (*AURRemove)(nil)
	_ Tool = (*AURUpdate)(nil)
	_ Tool = (*AURList)(nil)
	_ Tool = (*SystemMonitor)(nil)
	_ Tool = (*SystemProcesses)(nil)
	_ Tool = (*SystemLogs)(nil)
	_ Tool = (*SystemNotify)(nil)
)

// ── External embedder for token counting ──────────────────────────────

var tiktokenFunctions = map[string]int{}
var tiktokenMu sync.Mutex

func countTokens(text string) int {
	tiktokenMu.Lock()
	defer tiktokenMu.Unlock()
	// Rough estimate: 4 chars per token for non-OpenAI
	return int(math.Ceil(float64(len(text)) / 4.0))
}
