# Nala — Full Design Specification
### Go implementation • Self-hosting agent AI platform • Fully local, cloud-optional

---

## Table of Contents

- [0. Philosophy & Guiding Principles](#0-philosophy--guiding-principles)
- [1. System Architecture](#1-system-architecture)
- [2. Data Model](#2-data-model)
- [3. Configuration](#3-configuration)
- [4. Foundation Layer](#4-foundation-layer)
- [5. Model Integration](#5-model-integration)
- [6. Agent Runtime](#6-agent-runtime)
- [7. Tool System](#7-tool-system)
- [8. Memory & Knowledge](#8-memory--knowledge)
- [9. Multi-Agent & Orchestration](#9-multi-agent--orchestration)
- [10. User Interface](#10-user-interface)
- [11. Plugin System](#11-plugin-system)
- [12. API & Integration](#12-api--integration)
- [13. Security & Privacy](#13-security--privacy)
- [14. System & Deployment](#14-system--deployment)
- [15. Quality & Polish](#15-quality--polish)
- [16. Notes System](#16-notes-system)
- [17. Calendar & Reminders](#17-calendar--reminders)
- [18. Email Integration](#18-email-integration)
- [19. Skills System](#19-skills-system)
- [20. Editor (Markdown + LaTeX)](#20-editor-markdown--latex)
- [21. Personalization Engine](#21-personalization-engine)
- [22. Theme System](#22-theme-system)
- [23. System Monitoring & Hooks](#23-system-monitoring--hooks)
- [24. Safety & Guardrails](#24-safety--guardrails)
- [25. Automation & Triggers](#25-automation--triggers)
- [26. Data Structures (Go) — Full Reference](#26-data-structures-go--full-reference)
- [27. Implementation Order & Dependencies](#27-implementation-order--dependencies)
- [28. Success Metrics](#28-success-metrics)
- [29. Multi-Model Group Chat](#29-multi-model-group-chat)
- [30. What We Are NOT Building](#30-what-we-are-not-building)

---

## 0. Philosophy & Guiding Principles

The system models **agents as concurrent processes**, not sequential pipelines. An agent is a goroutine with a context, communicating via channels. Orchestration is a DAG of goroutines. Memory is a decay function over time.

### 0.1 Key Inversions vs Naive Approaches

- [ ] **No web app** — Desktop native (Wails), not Electron. Single binary
- [ ] **No docker required** — SQLite not Postgres. Everything embedded
- [ ] **No external runtime** — Binary bundles everything. No Python/Node runtime
- [ ] **Local-first** — Everything works offline. Cloud is optional addon
- [ ] **Privacy by design** — No telemetry. No analytics. No phone-home
- [ ] **API-first** — Every UI action has an API equivalent
- [ ] **Everything is a tool** — Agents, plugins, models can be tools
- [ ] **Desktop-native** — System tray, native menus, file dialogs
- [ ] **Fail gracefully** — Cache miss → fallback. Model down → fallback. Never crash

### 0.2 Architectural Principles

- [ ] No exceptions — all errors returned as values
- [ ] No global state — all state injected through constructors
- [ ] PascalCase identifiers (no snake_case, no ALL_CAPS)
- [ ] Tabs only for indentation (no spaces)
- [ ] Module headers with author/email on every file
- [ ] Every stage is a goroutine communicating via channels
- [ ] Graceful degradation — cache miss → fallback, never crash
- [ ] All API keys encrypted at rest (AES-256-GCM)
- [ ] Linux only (no macOS, no Windows)
- [ ] ASCII architecture diagram in docs
- [ ] Pipeline stage separation defined
- [ ] Cross-cutting concerns identified (storage, API, plugins)

### 0.3 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Nala Architecture                            │
│                                                                     │
│  ┌─────────────────────┐    ┌────────────────────────────────────┐  │
│  │    Wails Shell       │    │        Svelte Frontend             │  │
│  │  ┌─────────────────┐ │    │  ┌────────────────────────────┐  │  │
│  │  │ System Tray     │ │    │  │ Chat / Agent Config /      │  │  │
│  │  │ Native Menus    │◄┼────┼──┤ Pipeline Builder / Settings│  │  │
│  │  │ File Dialogs    │ │    │  │ Log Viewer / Dashboard     │  │  │
│  │  │ Notifications   │ │    │  └────────────────────────────┘  │  │
│  │  └─────────────────┘ │    └────────────────────────────────────┘  │
│  └──────────┬──────────┘                                             │
│             │ Wails Bindings (Go ↔ JS)                              │
│  ┌──────────▼─────────────────────────────────────────────────────┐ │
│  │                     Nala Core (Go)                              │ │
│  │                                                                 │ │
│  │  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │ │
│  │  │ Agent    │    │ Tool     │    │ Model    │    │ Memory   │ │ │
│  │  │ Runtime  │───>│ System   │───>│ Router   │───>│ Manager  │ │ │
│  │  └──────────┘    └──────────┘    └──────────┘    └──────────┘ │ │
│  │  ┌──────────┐    ┌──────────┐    ┌──────────┐                  │ │
│  │  │ Pipeline │    │Scheduler │    │ Plugin   │                  │ │
│  │  │ Engine   │    │ (cron)   │    │ Manager  │                  │ │
│  │  └──────────┘    └──────────┘    └──────────┘                  │ │
│  │                                                                 │ │
│  │  ┌──────────────────────────────────────────────────────────┐  │ │
│  │  │     Storage: SQLite + sqlite-vec + TOML                  │  │ │
│  │  └──────────────────────────────────────────────────────────┘  │ │
│  │  ┌──────────────────────────────────────────────────────────┐  │ │
│  │  │     API: REST (localhost:8472) + WebSocket + MCP         │  │ │
│  │  └──────────────────────────────────────────────────────────┘  │ │
│  └─────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 1. System Architecture

### 1.1 Process Model

- [ ] Root context with cancel created on startup
- [ ] errgroup.Group for API server goroutines
- [ ] errgroup.Group for agent manager goroutines
- [ ] Per-session child context with cancel
- [ ] Per-tool-call child context with timeout
- [ ] Context cancellation propagates: root → subsystem → session → tool
- [ ] All goroutines join before shutdown

### 1.2 Component Responsibilities

#### 1.2.1 Engine (internal/engine/engine.go)

- [ ] Loads configuration on startup
- [ ] Initializes logger (zap)
- [ ] Opens database (SQLite + migrations)
- [ ] Initializes model router (loads providers from DB)
- [ ] Initializes tool registry (registers built-in tools)
- [ ] Initializes agent manager
- [ ] Initializes memory manager
- [ ] Initializes pipeline engine
- [ ] Initializes scheduler (loads cron tasks from DB)
- [ ] Initializes plugin manager (loads plugins)
- [ ] Starts API server
- [ ] Starts scheduler
- [ ] Handles graceful shutdown (SIGINT, SIGTERM)
- [ ] Shutdown timeout: 30 seconds total
- [ ] Shutdown: drain API → stop scheduler → drain agents → close DB → flush logs

#### 1.2.2 Agent Runtime (internal/agent/)

- [ ] Creates agent sessions (goroutine per session)
- [ ] Runs conversation loop: user msg → model → tools → model → response
- [ ] Enforces per-agent timeouts
- [ ] Enforces per-session rate limits
- [ ] Persists session state after every message
- [ ] Handles pause/resume (context cancel + state save)

#### 1.2.3 Tool System (internal/tool/)

- [ ] Maintains tool registry with descriptions and JSON schemas
- [ ] Resolves tool calls from model output to executable functions
- [ ] Validates arguments against JSON Schema
- [ ] Enforces permission checks before execution
- [ ] Enforces timeouts per tool call
- [ ] Manages sandbox environments for untrusted tools

#### 1.2.4 Model Router (internal/model/)

- [ ] Abstracts LLM providers behind common interface
- [ ] Routes requests to appropriate provider based on agent config
- [ ] Implements fallback chains on provider error
- [ ] Manages context window (sliding window, truncation, summarization)
- [ ] Tracks token usage across sessions

#### 1.2.5 Memory Manager (internal/memory/)

- [ ] Manages conversation history (short-term)
- [ ] Extracts and stores user facts (long-term)
- [ ] Coordinates with vector store for RAG
- [ ] Runs periodic consolidation/summarization
- [ ] Full-text search across all entities

#### 1.2.6 Pipeline Engine (internal/pipeline/)

- [ ] Parses pipeline DAG definitions
- [ ] Schedules and executes agent steps in order
- [ ] Passes outputs between agents via variable substitution
- [ ] Handles branching, parallel execution, error recovery
- [ ] Tracks run status

#### 1.2.7 Scheduler (internal/scheduler/)

- [ ] Parses cron expressions
- [ ] Triggers agent runs at scheduled times
- [ ] Supports catch-up for missed runs

#### 1.2.8 Notes Manager (internal/notes/)

- [ ] Manages note CRUD (files + DB metadata)
- [ ] Handles folder hierarchy (unlimited nesting)
- [ ] Runs FTS5 search across notes
- [ ] Syncs DB ↔ filesystem on startup
- [ ] Manages backlinks between notes
- [ ] Handles TODO extraction from notes
- [ ] Manages trash with 30-day retention
- [ ] Handles attachment storage and cleanup

#### 1.2.9 Calendar Manager (internal/calendar/)

- [ ] Manages local and remote calendars
- [ ] Handles event CRUD with recurrence (RRULE)
- [ ] Expands recurring events at query time
- [ ] Detects event overlaps on create/update
- [ ] Manages timezone conversion (UTC storage, local display)
- [ ] Handles CalDAV sync (read/write)
- [ ] Manages reminders with snooze and missed detection

#### 1.2.10 Email Manager (internal/email/)

- [ ] Manages IMAP/SMTP accounts with encrypted credentials
- [ ] Runs periodic IMAP sync (UID-based incremental)
- [ ] Handles SMTP sending with queue and retry
- [ ] Groups messages into threads by Message-ID
- [ ] Runs FTS5 search across all email fields
- [ ] Evaluates filter rules on new messages
- [ ] Manages PGP encryption/decryption

#### 1.2.11 Skills Manager (internal/skills/)

- [ ] Manages skill lifecycle (install, enable, disable, uninstall, update)
- [ ] Validates skill manifests on install
- [ ] Runs skills in sandboxed environments (Lua, WASM, Python)
- [ ] Enforces skill permissions
- [ ] Checks for skill updates (auto-update)
- [ ] Resolves skill dependencies

#### 1.2.12 Editor Service (internal/editor/)

- [ ] Manages file operations (open, save, rename, delete)
- [ ] Handles CodeMirror 6 integration via Wails
- [ ] Manages tab state persistence
- [ ] Tracks recent files
- [ ] Manages auto-save timer

#### 1.2.13 Monitor Service (internal/monitor/)

- [ ] Collects system metrics (CPU, memory, disk, network, GPU)
- [ ] Maintains ring buffer for recent history
- [ ] Persists historical data to DB (sampled)
- [ ] Evaluates alert thresholds
- [ ] Manages system event hooks (inotify, process, etc.)

#### 1.2.14 Personalization Engine (internal/personalization/)

- [ ] Builds and maintains user profile
- [ ] Infers preferences from conversation and behavior
- [ ] Adjusts model persona based on profile
- [ ] Manages privacy controls and opt-out
- [ ] Provides content recommendations

#### 1.2.15 Automation Engine (internal/automation/)

- [ ] Manages trigger definitions (cron, file, webhook, metric, etc.)
- [ ] Evaluates trigger conditions with AND/OR logic
- [ ] Executes actions (run agent, script, webhook, notification)
- [ ] Handles action chaining with conditional flow
- [ ] Manages failure handling with retry and backoff

### 1.3 Project Structure

- [ ] `cmd/nala/main.go` — Desktop app entry
- [ ] `cmd/nalad/main.go` — Headless server entry
- [ ] `internal/engine/` — Engine
- [ ] `internal/agent/` — Agent runtime
- [ ] `internal/model/` — Model providers
- [ ] `internal/tool/` — Tool system
- [ ] `internal/memory/` — Memory & knowledge
- [ ] `internal/pipeline/` — Pipeline engine
- [ ] `internal/scheduler/` — Cron scheduler
- [ ] `internal/plugin/` — Plugin system
- [ ] `internal/api/` — REST API
- [ ] `internal/db/` — Database layer
- [ ] `internal/config/` — Configuration
- [ ] `internal/crypto/` — Encryption
- [ ] `internal/logger/` — Logging
- [ ] `internal/notes/` — Notes system
- [ ] `internal/calendar/` — Calendar & reminders
- [ ] `internal/email/` — Email integration
- [ ] `internal/skills/` — Skills system
- [ ] `internal/editor/` — Editor service
- [ ] `internal/monitor/` — System monitoring
- [ ] `internal/personalization/` — Personalization engine
- [ ] `internal/automation/` — Automation & triggers
- [ ] `ui/` — Svelte frontend
- [ ] `docs/` — Documentation
- [ ] `Makefile` — Build targets
- [ ] `wails.json` — Wails config
- [ ] `.golangci.yml` — Linter config
---

## 2. Data Model

### 2.1 Entities

- [x] Agent entity defined
- [x] Session entity defined
- [x] Message entity defined
- [x] ToolBinding entity defined
- [x] KnowledgeBase entity defined
- [x] Document entity defined
- [x] ProviderConfig entity defined
- [x] ScheduledTask entity defined
- [x] AuditLogEntry entity defined
- [ ] Task entity defined
- [ ] Note entity defined
- [ ] Calendar entity defined
- [ ] Email entity defined
- [ ] Reminder entity defined
- [ ] Skill entity defined
- [ ] UserData entity defined
- [ ] SystemHook entity defined
- [ ] ERD: Agent 1:N Session 1:N Message
- [ ] ERD: KB 1:N Document
- [ ] ERD: Agent N:N ToolBinding
- [ ] ERD: Calendar 1:N Event
- [ ] ERD: Event 1:N Reminder

### 2.2 Agent Table

**SQL:**
```sql
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    description     TEXT DEFAULT '',
    system_prompt   TEXT DEFAULT '',
    model_config    TEXT NOT NULL DEFAULT '{}',
    tool_bindings   TEXT NOT NULL DEFAULT '[]',
    memory_config   TEXT NOT NULL DEFAULT '{}',
    max_tokens      INTEGER DEFAULT 4096,
    temperature     REAL DEFAULT 0.7,
    top_p           REAL DEFAULT 0.9,
    presence_penalty REAL DEFAULT 0.0,
    frequency_penalty REAL DEFAULT 0.0,
    personality     TEXT DEFAULT 'default',
    timeout_ms      INTEGER DEFAULT 300000,
    max_retries     INTEGER DEFAULT 3,
    metadata        TEXT DEFAULT '{}',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_agents_slug ON agents(slug);
CREATE INDEX idx_agents_created ON agents(created_at);
```

- [ ] SQL: agents table with all columns
- [ ] SQL: idx_agents_slug UNIQUE index
- [ ] SQL: idx_agents_created index
- [x] Go: `type Agent struct` defined
- [ ] Validation: name required, 1-100 chars
- [ ] Validation: slug matches `^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$`
- [ ] Validation: temperature in [0.0, 2.0]
- [ ] Validation: top_p in [0.0, 1.0]
- [ ] Validation: timeout_ms in [1000, 600000]
- [ ] Validation: max_retries in [0, 10]
- [ ] Edge case: empty system prompt (default behavior)
- [ ] Edge case: empty tool bindings (no tools)
- [ ] Edge case: invalid personality key (fall back to "default")
- [ ] Edge case: slug collision (auto-append number)
- [ ] Test: create → get returns same agent
- [ ] Test: validation rejects invalid values
- [ ] Test: duplicate slug errors

### 2.3 Session Table

```sql
CREATE TABLE sessions (
    id                TEXT PRIMARY KEY,
    agent_id          TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    title             TEXT DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'active',
    context_size      INTEGER DEFAULT 0,
    max_context_size  INTEGER DEFAULT 128000,
    message_count     INTEGER DEFAULT 0,
    total_tokens_in   INTEGER DEFAULT 0,
    total_tokens_out  INTEGER DEFAULT 0,
    total_cost        REAL DEFAULT 0.0,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    paused_at         TEXT,
    completed_at      TEXT
);
CREATE INDEX idx_sessions_agent ON sessions(agent_id);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_updated ON sessions(updated_at);
```

- [ ] SQL: sessions table with all columns
- [ ] SQL: FK with ON DELETE CASCADE
- [ ] SQL: indexes on agent_id, status, updated_at
- [x] Go: `type Session struct` defined
- [ ] Session status: active, paused, completed, expired, error
- [ ] Edge case: status transitions (created→active→paused→completed)
- [ ] Edge case: pause already-paused session (no-op)
- [ ] Edge case: resume active session (no-op)
- [ ] Edge case: delete agent with active sessions (cascade)
- [ ] Test: create → pause → resume cycle
- [ ] Test: session expiry deletes old sessions

### 2.4 Message Table

```sql
CREATE TABLE messages (
    id            TEXT PRIMARY KEY,
    session_id    TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    parent_id     TEXT REFERENCES messages(id),
    role          TEXT NOT NULL,
    content       TEXT,
    tool_calls    TEXT DEFAULT '[]',
    tool_results  TEXT DEFAULT '[]',
    model         TEXT,
    tokens_in     INTEGER DEFAULT 0,
    tokens_out    INTEGER DEFAULT 0,
    duration_ms   INTEGER,
    error         TEXT,
    metadata      TEXT DEFAULT '{}',
    created_at    TEXT NOT NULL
);
CREATE INDEX idx_messages_session ON messages(session_id);
CREATE INDEX idx_messages_created ON messages(session_id, created_at);
```

- [ ] SQL: messages table with all columns
- [ ] SQL: indexes
- [x] Go: `type Message struct` defined
- [x] Go: `type ToolCall struct` defined
- [x] Go: `type ToolResult struct` defined
- [ ] Role values: system, user, assistant, tool
- [ ] Edge case: content=NULL when only tool_calls
- [ ] Edge case: very long content >100KB (truncate)
- [ ] Edge case: tool call with invalid JSON arguments
- [ ] Edge case: assistant msg with both text AND tool_calls
- [ ] Test: insert and retrieve messages in order
- [ ] Test: cascade delete with session

### 2.5 ProviderConfig Table

```sql
CREATE TABLE provider_configs (
    id                 TEXT PRIMARY KEY,
    provider           TEXT NOT NULL,
    display_name       TEXT,
    endpoint           TEXT,
    api_key_encrypted  TEXT,
    api_key_hint       TEXT,
    models             TEXT DEFAULT '[]',
    default_model      TEXT,
    priority           INTEGER DEFAULT 0,
    enabled            INTEGER NOT NULL DEFAULT 1,
    rate_limit_rpm     INTEGER,
    rate_limit_tpm     INTEGER,
    timeout_ms         INTEGER DEFAULT 60000,
    max_retries        INTEGER DEFAULT 3,
    metadata           TEXT DEFAULT '{}',
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);
CREATE INDEX idx_providers_type ON provider_configs(provider);
CREATE INDEX idx_providers_priority ON provider_configs(priority);
```

- [ ] SQL: provider_configs table
- [ ] SQL: indexes
- [x] Go: `type ProviderConfig struct` defined
- [ ] Provider values: ollama, openai, anthropic, google, mistral, groq, generic
- [ ] API key encrypted at rest (AES-256-GCM)
- [ ] API key hint: last 4 chars ("…ab12")
- [ ] Edge case: empty endpoint for local providers (ollama)
- [ ] Edge case: all providers disabled (no models available)
- [ ] Edge case: duplicate display name (warn)
- [ ] Test: provider CRUD
- [ ] Test: API key encryption/decryption

### 2.6 KnowledgeBase Table

```sql
CREATE TABLE knowledge_bases (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    description       TEXT DEFAULT '',
    embedding_model   TEXT NOT NULL,
    chunk_strategy    TEXT NOT NULL DEFAULT 'recursive',
    chunk_size        INTEGER DEFAULT 1000,
    chunk_overlap     INTEGER DEFAULT 200,
    document_count    INTEGER DEFAULT 0,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
```

- [ ] SQL: knowledge_bases table
- [x] Go: `type KnowledgeBase struct` defined
- [ ] Chunk strategies: fixed, recursive, semantic
- [ ] Edge case: empty KB (no documents)
- [ ] Edge case: delete KB with documents (cascade)
- [ ] Edge case: embedding model not available (error on create)
- [ ] Test: KB CRUD

### 2.7 Document Table

```sql
CREATE TABLE documents (
    id            TEXT PRIMARY KEY,
    kb_id         TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    content       TEXT,
    content_hash  TEXT,
    mime_type     TEXT,
    chunks        TEXT DEFAULT '[]',
    metadata      TEXT DEFAULT '{}',
    created_at    TEXT NOT NULL
);
CREATE INDEX idx_documents_kb ON documents(kb_id);
CREATE INDEX idx_documents_hash ON documents(content_hash);
```

- [ ] SQL: documents table
- [ ] SQL: indexes
- [x] Go: `type Document struct` defined
- [x] Go: `type Chunk struct` defined
- [ ] Edge case: binary file with no content (metadata only)
- [ ] Edge case: encrypted PDF (error)
- [ ] Edge case: duplicate file (content_hash → skip)
- [ ] Edge case: very large file >100MB (warn, background)
- [ ] Test: insert and retrieve document
- [ ] Test: duplicate detection

### 2.8 ScheduledTask Table

```sql
CREATE TABLE scheduled_tasks (
    id                TEXT PRIMARY KEY,
    agent_id          TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    cron_expression   TEXT NOT NULL,
    input_template    TEXT DEFAULT '',
    enabled           INTEGER NOT NULL DEFAULT 1,
    last_run_at       TEXT,
    last_run_status   TEXT,
    next_run_at       TEXT,
    max_runs          INTEGER DEFAULT 0,
    run_count         INTEGER DEFAULT 0,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_scheduled_next ON scheduled_tasks(enabled, next_run_at);
```

- [ ] SQL: scheduled_tasks table
- [ ] SQL: index
- [x] Go: `type ScheduledTask struct` defined
- [ ] Cron: standard 5-field
- [ ] Edge case: next_run_at in past on startup (catch-up)
- [ ] Edge case: max_runs=0 means unlimited
- [ ] Edge case: invalid cron expression (validation error)
- [ ] Test: task CRUD

### 2.9 AuditLog Table

```sql
CREATE TABLE audit_log (
    id            TEXT PRIMARY KEY,
    timestamp     TEXT NOT NULL,
    level         TEXT NOT NULL,
    category      TEXT NOT NULL,
    action        TEXT NOT NULL,
    actor_id      TEXT,
    actor_type    TEXT,
    session_id    TEXT,
    details       TEXT DEFAULT '{}',
    duration_ms   INTEGER
);
CREATE INDEX idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX idx_audit_category ON audit_log(category);
CREATE INDEX idx_audit_actor ON audit_log(actor_id);
```

- [ ] SQL: audit_log table
- [ ] SQL: indexes
- [x] Go: `type AuditLogEntry struct` defined
- [ ] Level: debug, info, warn, error
- [ ] Category: tool_exec, model_call, agent_action, config_change, auth, system
- [ ] Retention: configurable (default 90 days)
- [ ] Auto-prune on startup
- [ ] Edge case: huge detail payload (truncate at 10KB)
- [ ] Test: insert and query audit logs
- [ ] Test: retention/pruning

### 2.10 Tasks Table

```sql
CREATE TABLE tasks (
    id                 TEXT PRIMARY KEY,
    title              TEXT NOT NULL,
    description        TEXT DEFAULT '',
    status             TEXT NOT NULL DEFAULT 'pending',
    priority           TEXT NOT NULL DEFAULT 'medium',
    due_date           TEXT,
    completed_at       TEXT,
    assigned_agent_id  TEXT REFERENCES agents(id),
    linked_entity_type TEXT,
    linked_entity_id   TEXT,
    metadata           TEXT DEFAULT '{}',
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_due ON tasks(due_date);
CREATE INDEX idx_tasks_linked ON tasks(linked_entity_type, linked_entity_id);
```

- [ ] SQL: tasks table
- [ ] SQL: indexes
- [ ] Go: `type Task struct` defined
- [ ] Status values: pending, in_progress, completed, cancelled, deferred
- [ ] Priority values: low, medium, high, urgent, critical
- [ ] Edge case: empty title (validation error)
- [ ] Edge case: due_date in past on create (warn)
- [ ] Edge case: task linked to deleted entity (orphaned, warn)
- [ ] Edge case: 10000+ tasks (paginate, virtual list)
- [ ] Test: create → complete task
- [ ] Test: filter by status
- [ ] Test: linked entity reference

### 2.11 Notes Table

```sql
CREATE TABLE notes (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    folder            TEXT DEFAULT '',
    tags              TEXT DEFAULT '[]',
    pinned            INTEGER DEFAULT 0,
    color             TEXT,
    word_count        INTEGER DEFAULT 0,
    content_hash      TEXT,
    file_path         TEXT NOT NULL,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_notes_folder ON notes(folder);
CREATE INDEX idx_notes_pinned ON notes(pinned);
```

- [ ] SQL: notes table
- [ ] SQL: indexes
- [ ] Go: `type Note struct` defined
- [ ] file_path is relative to `{data_dir}/notes/`
- [ ] Edge case: note with same title in same folder (allow, dedup view)
- [ ] Edge case: very long title >500 chars (truncate)
- [ ] Edge case: folder path with special characters (escape)
- [ ] Edge case: note file deleted externally (detect, recreate from DB)
- [ ] Edge case: 10000+ notes in single folder (pagination)
- [ ] Test: create note → DB record + file
- [ ] Test: update note title → file renamed
- [ ] Test: delete note → moved to trash

### 2.12 Calendars & Events Tables

```sql
CREATE TABLE calendars (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    description       TEXT DEFAULT '',
    color             TEXT DEFAULT '#4A90D9',
    sync_url          TEXT,
    sync_type         TEXT NOT NULL DEFAULT 'local',
    credentials       TEXT,
    enabled           INTEGER NOT NULL DEFAULT 1,
    last_synced_at    TEXT,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE TABLE events (
    id                  TEXT PRIMARY KEY,
    calendar_id         TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    description         TEXT DEFAULT '',
    location            TEXT DEFAULT '',
    start_time          TEXT NOT NULL,
    end_time            TEXT,
    all_day             INTEGER DEFAULT 0,
    timezone            TEXT DEFAULT 'UTC',
    rrule               TEXT,
    recurrence_exceptions TEXT DEFAULT '[]',
    color               TEXT,
    status              TEXT NOT NULL DEFAULT 'confirmed',
    metadata            TEXT DEFAULT '{}',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX idx_events_calendar ON events(calendar_id);
CREATE INDEX idx_events_start ON events(start_time);
CREATE INDEX idx_events_end ON events(end_time);
```

- [ ] SQL: calendars and events tables
- [ ] SQL: indexes
- [ ] Go: `type Calendar struct` defined
- [ ] Go: `type Event struct` defined
- [ ] Sync types: local, caldav, google
- [ ] Event status: confirmed, tentative, cancelled
- [ ] Edge case: recurring event with no end (expand 2 years max)
- [ ] Edge case: event spanning DST transition (correct conversion)
- [ ] Edge case: orphaned calendar after account removal (cascade delete)
- [ ] Edge case: duplicate event from sync (dedup by UID)
- [ ] Test: create event in calendar
- [ ] Test: recurring event expansion
- [ ] Test: cascade delete with calendar

### 2.13 Emails Table

```sql
CREATE TABLE email_accounts (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    email              TEXT NOT NULL,
    imap_host          TEXT,
    imap_port          INTEGER DEFAULT 993,
    imap_username      TEXT,
    imap_password_enc  TEXT,
    smtp_host          TEXT,
    smtp_port          INTEGER DEFAULT 587,
    smtp_username      TEXT,
    smtp_password_enc  TEXT,
    oauth_token_enc    TEXT,
    provider           TEXT DEFAULT 'other',
    status             TEXT NOT NULL DEFAULT 'disconnected',
    sync_enabled       INTEGER DEFAULT 1,
    last_synced_uid    INTEGER DEFAULT 0,
    metadata           TEXT DEFAULT '{}',
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);

CREATE TABLE emails (
    id                TEXT PRIMARY KEY,
    account_id        TEXT NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    message_id        TEXT,
    thread_id         TEXT,
    folder            TEXT NOT NULL DEFAULT 'INBOX',
    uid               INTEGER,
    from_addr         TEXT NOT NULL,
    to_addr           TEXT NOT NULL,
    cc_addr           TEXT DEFAULT '',
    bcc_addr          TEXT DEFAULT '',
    subject           TEXT DEFAULT '',
    body_text         TEXT,
    body_html         TEXT,
    is_read           INTEGER DEFAULT 0,
    is_starred        INTEGER DEFAULT 0,
    is_encrypted      INTEGER DEFAULT 0,
    has_attachments   INTEGER DEFAULT 0,
    received_at       TEXT NOT NULL,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL
);
CREATE INDEX idx_emails_account ON emails(account_id);
CREATE INDEX idx_emails_thread ON emails(thread_id);
CREATE INDEX idx_emails_folder ON emails(account_id, folder);
CREATE INDEX idx_emails_received ON emails(received_at);
CREATE INDEX idx_emails_message_id ON emails(message_id);
```

- [ ] SQL: email_accounts and emails tables
- [ ] SQL: indexes
- [ ] Go: `type EmailAccount struct` defined
- [ ] Go: `type Email struct` defined
- [ ] Credentials encrypted at rest via AES-256-GCM
- [ ] Edge case: duplicate email (dedup by Message-ID)
- [ ] Edge case: email with no body (try body_html)
- [ ] Edge case: very long subject >998 chars (truncate per RFC 2822)
- [ ] Edge case: account with 100K+ messages (lazy load, pagination)
- [ ] Test: account CRUD
- [ ] Test: email insert and query
- [ ] Test: credential encryption round-trip

### 2.14 Reminders Table

```sql
CREATE TABLE reminders (
    id                TEXT PRIMARY KEY,
    event_id          TEXT REFERENCES events(id) ON DELETE CASCADE,
    title             TEXT NOT NULL,
    remind_at         TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending',
    snoozed_until     TEXT,
    dismissed_at      TEXT,
    sound             TEXT,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_reminders_remind ON reminders(remind_at, status);
CREATE INDEX idx_reminders_event ON reminders(event_id);
```

- [ ] SQL: reminders table
- [ ] SQL: indexes
- [ ] Go: `type Reminder struct` defined
- [ ] Status values: pending, snoozed, fired, dismissed, missed
- [ ] Edge case: reminder for deleted event (cascade delete)
- [ ] Edge case: 50+ reminders firing at once (batch, group notification)
- [ ] Edge case: remind_at in past on startup (missed detection)
- [ ] Test: create and fire reminder
- [ ] Test: snooze reschedules correctly

### 2.15 Skills Table

```sql
CREATE TABLE skills (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    version           TEXT NOT NULL,
    description       TEXT DEFAULT '',
    author            TEXT DEFAULT '',
    license           TEXT DEFAULT '',
    category          TEXT NOT NULL,
    language          TEXT NOT NULL,
    entrypoint        TEXT NOT NULL,
    permissions       TEXT DEFAULT '{}',
    config_schema     TEXT DEFAULT '{}',
    dependencies      TEXT DEFAULT '{}',
    is_enabled        INTEGER DEFAULT 0,
    is_builtin        INTEGER DEFAULT 0,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_skills_category ON skills(category);
CREATE INDEX idx_skills_enabled ON skills(is_enabled);
```

- [ ] SQL: skills table
- [ ] SQL: indexes
- [ ] Go: `type Skill struct` defined
- [ ] Go: `type SkillManifest struct` defined
- [ ] Languages: lua, python, wasm
- [ ] Categories: coding, writing, research, productivity, entertainment, utility
- [ ] Edge case: skill with missing entrypoint (error on enable)
- [ ] Edge case: built-in skill cannot be uninstalled
- [ ] Edge case: two skills with conflicting tool IDs (first-enable wins)
- [ ] Edge case: skill dependency not met (block enable)
- [ ] Test: install → enable → disable → uninstall cycle
- [ ] Test: built-in skill persists across reinstall

### 2.16 UserData Table

```sql
CREATE TABLE user_data (
    id                TEXT PRIMARY KEY,
    category          TEXT NOT NULL,
    key               TEXT NOT NULL,
    value             TEXT NOT NULL,
    confidence        REAL DEFAULT 1.0,
    source            TEXT DEFAULT 'explicit',
    is_locked         INTEGER DEFAULT 0,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_user_data_category ON user_data(category);
CREATE INDEX idx_user_data_key ON user_data(category, key);
```

- [ ] SQL: user_data table
- [ ] SQL: indexes
- [ ] Go: `type UserData struct` defined
- [ ] Categories: preference, fact, behavior, stat
- [ ] Sources: explicit, implicit_conversation, implicit_tool, implicit_feedback
- [ ] Confidence: 0.0 (guess) to 1.0 (explicitly set)
- [ ] Edge case: conflicting values for same key (highest confidence wins)
- [ ] Edge case: privacy opt-out prevents collection
- [ ] Edge case: key with very long value >10KB (truncate)
- [ ] Test: insert and retrieve user data
- [ ] Test: confidence-based conflict resolution

### 2.17 SystemHooks Table

```sql
CREATE TABLE system_hooks (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    description       TEXT DEFAULT '',
    trigger_type      TEXT NOT NULL,
    trigger_config    TEXT NOT NULL DEFAULT '{}',
    action_type       TEXT NOT NULL,
    action_config     TEXT NOT NULL DEFAULT '{}',
    conditions        TEXT DEFAULT '{}',
    cooldown_seconds  INTEGER DEFAULT 0,
    max_fires         INTEGER DEFAULT 0,
    fire_count        INTEGER DEFAULT 0,
    last_fired_at     TEXT,
    error_count       INTEGER DEFAULT 0,
    enabled           INTEGER NOT NULL DEFAULT 1,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_hooks_trigger ON system_hooks(trigger_type);
CREATE INDEX idx_hooks_enabled ON system_hooks(enabled);
```

- [ ] SQL: system_hooks table
- [ ] SQL: indexes
- [ ] Go: `type SystemHook struct` defined
- [ ] Trigger types: schedule, file_change, process_event, system_metric, webhook, time_interval, agent_completion, email_received, calendar_event, startup
- [ ] Action types: run_agent, execute_pipeline, run_script, send_notification, send_email, webhook_call, log
- [ ] Edge case: hook with max_fires=0 (unlimited)
- [ ] Edge case: hook auto-disabled after max_errors
- [ ] Edge case: circular trigger chain (detect, break)
- [ ] Edge case: webhook with invalid secret (403, log)
- [ ] Test: hook CRUD
- [ ] Test: trigger fires action

### 2.18 Migration Strategy

- [ ] Migration files in `internal/db/migrations/`
- [ ] Numbered sequentially (001_, 002_, ...)
- [ ] `_migrations` table tracks applied
- [ ] Applied on startup in order
- [ ] Each wrapped in transaction
- [ ] Idempotent: only unapplied run
- [x] Migration 001: create agents table
- [x] Migration 002: create sessions table
- [x] Migration 003: create messages table
- [x] Migration 004: create tool_bindings table
- [x] Migration 005: create knowledge_bases table
- [x] Migration 006: create documents table
- [x] Migration 007: create provider_configs table
- [x] Migration 008: create scheduled_tasks table
- [x] Migration 009: create audit_log table
- [x] Migration 010: create tasks table
- [x] Migration 011: create notes table
- [ ] Migration 012: create calendars table
- [ ] Migration 013: create events table
- [ ] Migration 014: create email_accounts table
- [ ] Migration 015: create emails table
- [ ] Migration 016: create reminders table
- [ ] Migration 017: create skills table
- [ ] Migration 018: create user_data table
- [ ] Migration 019: create system_hooks table
- [ ] Migration 020: create FTS5 indexes (notes, emails)
- [ ] Migration 021: load sqlite-vec extension
- [ ] Test: all migrations apply cleanly on fresh DB
- [ ] Test: re-running is idempotent

---

## 3. Configuration

### 3.1 Config File

- [ ] Location: `~/.config/nala/config.toml`
- [ ] Auto-created with defaults on first run
- [ ] TOML format via `BurntSushi/toml`
- [ ] All values overridable via `NALA_` env vars

### 3.2 Core Config Fields

- [ ] `core.data_dir` — string, `~/.local/share/nala/`
- [ ] `core.log_level` — string, `info`, validated (debug|info|warn|error)
- [ ] `core.log_file` — string, default empty (stdout)
- [ ] `core.log_max_size` — int, 50 MB
- [ ] `core.log_max_age` — int, 30 days
- [ ] `core.max_sessions` — int, 10 (1-100)
- [ ] `core.max_upload_size_mb` — int, 100 (1-1000)
- [ ] `server.enabled` — bool, true
- [ ] `server.host` — string, `127.0.0.1`
- [ ] `server.port` — int, 8472 (1024-65535)
- [ ] `server.require_auth` — bool, false
- [ ] `server.cors_origins` — string[], `["http://localhost:*","wails://*"]`
- [ ] `server.rate_limit_rpm` — int, 60
- [ ] `model.default_provider` — string, `ollama`
- [ ] `model.default_model` — string, `llama3.2:3b`
- [ ] `model.max_tokens` — int, 4096
- [ ] `model.timeout_s` — int, 120
- [ ] `model.max_concurrent` — int, 4
- [ ] `memory.vector_backend` — string, `sqlite-vec`
- [ ] `memory.default_chunk_size` — int, 1000
- [ ] `memory.default_chunk_overlap` — int, 200
- [ ] `memory.summarization_interval_min` — int, 60
- [ ] `memory.auto_extract_facts` — bool, true
- [ ] `tools.sandbox_dir` — string, `~/.local/share/nala/sandbox`
- [ ] `tools.code_exec_timeout_s` — int, 30
- [ ] `tools.allowed_languages` — string[], `["python","javascript","go","bash"]`
- [ ] `tools.tools_network_access` — bool, false
- [ ] `tools.max_concurrent_tools` — int, 8
- [ ] `privacy.airgap_mode` — bool, false
- [ ] `privacy.audit_retention_days` — int, 90
- [ ] `privacy.sanitize_uploads` — bool, true
- [ ] `privacy.disable_telemetry` — bool, true
- [ ] `privacy.session_expiry_days` — int, 30
- [ ] `ui.theme` — string, `system`, validated (light|dark|system)
- [ ] `ui.font_size` — int, 14
- [ ] `ui.language` — string, `en`, validated (en|ja)
- [ ] `theme.name` — string, `catppuccin-mocha`
- [ ] `theme.mode` — string, `system`, validated (light|dark|system)
- [ ] `theme.opacity` — float, 1.0 (0.0-1.0)
- [ ] `theme.wallpaper` — string, empty
- [ ] `theme.blur_intensity` — int, 0 (0-20)
- [ ] `theme.font_family` — string, system default
- [ ] `theme.font_size` — int, 14 (10-32)
- [ ] `theme.line_height` — float, 1.5 (1.0-2.5)
- [ ] `theme.animations` — bool, true
- [ ] `theme.animation_speed` — string, `normal`, validated (none|slow|normal|fast)
- [ ] `calendar.default_view` — string, `month`, validated (month|week|day|agenda)
- [ ] `calendar.week_start` — string, `monday`, validated (sunday|monday)
- [ ] `calendar.working_hours_start` — int, 9 (0-23)
- [ ] `calendar.working_hours_end` — int, 17 (0-23)
- [ ] `calendar.sync_interval_min` — int, 15
- [ ] `calendar.default_reminder_min` — int, 10
- [ ] `calendar.show_week_numbers` — bool, false
- [ ] `email.sync_interval_s` — int, 60
- [ ] `email.max_attachment_mb` — int, 25
- [ ] `email.blocked_extensions` — string[], `[".exe",".bat",".cmd",".scr",".vbs"]`
- [ ] `email.pgp_enabled` — bool, false
- [ ] `email.default_fetch_count` — int, 50
- [ ] `skills.store_url` — string, `https://skills.nala.ai/`
- [ ] `skills.auto_update` — bool, true
- [ ] `skills.max_concurrent` — int, 4
- [ ] `skills.default_timeout_s` — int, 30
- [ ] `personalization.enabled` — bool, true
- [ ] `personalization.inference_interval` — int, 10
- [ ] `personalization.privacy_level` — string, `medium`, validated (none|low|medium|high|maximum)
- [ ] `personalization.learning_sources` — string[], `["conversation","tools","feedback"]`

### 3.3 Env Var Overrides

- [ ] Pattern: `NALA_{SECTION}_{KEY}`
- [ ] Nested: `NALA_SECTION__NESTED__KEY`
- [ ] Arrays: comma-separated
- [ ] Booleans: "true" / "false"
- [ ] CLI flags override env vars override config file

### 3.4 Config Validation

- [ ] Invalid log_level → warn + use default
- [ ] Invalid port → error + exit
- [ ] Invalid data_dir → error + exit
- [ ] All enum fields validated against allowed values
- [ ] Numeric fields validated against min/max
- [ ] Hot-reload: safe fields apply immediately (log_level, theme, font_size)
- [ ] Unsafe fields require restart (port, data_dir)
- [ ] Test: each invalid value produces correct error
- [ ] Test: each valid value accepted
- [ ] Test: env vars override config file

---

## 4. Foundation Layer

### 4.1 NALA-001: Project Scaffolding

- [x] Go module: `github.com/KleaSCM/nala`
- [x] Go version: 1.24+
- [x] `wails.json` with app metadata
- [x] App ID: `com.kleascm.nala`
- [x] Window: single, resizable, min 800×600, default 1200×800
- [x] Window title: "Nala"
- [x] System tray with icon
- [x] Svelte 5 + Vite + TypeScript strict mode
- [x] Tailwind CSS v4 with custom theme
- [x] `Makefile` targets: dev, build, build-all, test, lint, clean
- [x] `make dev` launches app with hot reload
- [x] `make build` produces single binary in `build/`
- [x] `golangci-lint` config
- [x] Prettier + ESLint config for UI
- [x] `.gitignore` for Go + Node + Wails artifacts

### 4.2 NALA-002: Window Shell

- [x] Window opens at 1200×800, centered
- [x] Minimum size: 800×600
- [x] Window position/size persisted between launches
- [ ] Closing window hides to tray (configurable)
- [x] System tray icon (cat silhouette)
- [ ] Tray menu: Show Nala, New Chat, Recent Agents (5), Settings, Quit
- [x] Native menu: Nala (About, Settings, Quit)
- [x] Native menu: Edit (Undo, Redo, Cut, Copy, Paste, Select All)
- [x] Native menu: View (Toggle Sidebar, Toggle DevTools, Full Screen, Reload)
- [x] Native menu: Help (Documentation, Report Issue, About)
- [ ] Notifications: agent completed, scheduled run, error, update available
- [ ] Native file dialogs via Wails bridge

### 4.3 NALA-003: Configuration System

- [x] `config.Load()` function
- [x] `config.Get()` returns current config
- [x] Config file created with defaults on first run
- [x] Environment variable overlay
- [x] Config validation on load
- [x] Config hot-reload via `fsnotify`
- [x] Safe fields apply immediately
- [x] Unsafe fields require restart
- [x] UI notification on config change
- [x] Test: Load() returns defaults
- [x] Test: Load() overrides from env vars

### 4.4 NALA-004: SQLite Database

- [x] Path: `{data_dir}/nala.db`
- [x] Driver: `modernc.org/sqlite` (pure Go, no CGo)
- [x] WAL journal mode
- [x] Busy timeout: 5000ms
- [x] Foreign keys ON
- [x] Synchronous: NORMAL
- [x] Cache: 8MB
- [x] Migration runner in `internal/db/migrate.go`
- [x] `_migrations` table tracking
- [x] All migrations wrapped in transactions
- [x] Test: NewDB() creates file
- [x] Test: Migrate() applies all
- [x] Test: Migrate() is idempotent

### 4.5 NALA-005: Logging

- [x] Logger wrapper around `go.uber.org/zap`
- [x] Methods: Debug, Info, Warn, Error
- [x] `With(fields)` returns child logger
- [x] JSON output format
- [x] stdout output (default)
- [x] File output when `log_file` set
- [x] Log rotation: 50MB, 5 files, gzip, 30 days
- [ ] UI log viewer component
- [ ] Log viewer: real-time via Wails events
- [ ] Log viewer: filter by level, search by text
- [ ] Log viewer: color-coded levels
- [ ] Log viewer: auto-scroll with pause
- [ ] Log viewer: export (JSON, plain text)
- [ ] Test: Logger writes correct JSON
- [ ] Test: With() attaches fields

### 4.6 NALA-006: Graceful Shutdown

- [x] SIGINT handler registered
- [x] SIGTERM handler registered
- [x] Cancel root context
- [x] API server drain (10s timeout)
- [x] Agent sessions drain (15s timeout)
- [x] In-flight model calls wait (10s)
- [x] Plugin manager stop
- [x] Database close
- [x] Logger flush
- [x] Total timeout: 30s
- [x] After timeout: os.Exit(1)
- [x] Test: SIGINT triggers graceful shutdown
- [x] Test: force exit after timeout

---

## 5. Model Integration

### 5.1 Provider Interface

- [x] `Provider.ID() string`
- [x] `Provider.Name() string`
- [x] `Provider.ListModels(ctx) ([]ModelInfo, error)`
- [x] `Provider.Chat(ctx, ChatRequest) (*ChatResponse, error)`
- [x] `Provider.ChatStream(ctx, ChatRequest) (<-chan StreamDelta, error)`
- [x] `Provider.CountTokens(ctx, msgs) (int, error)`
- [x] `Provider.ValidateConfig() error`
- [x] `type ChatRequest struct` — Model, Messages, SystemPrompt, Parameters, Tools, Stream
- [x] `type ChatResponse struct` — Message, Usage, Model, Duration
- [x] `type StreamDelta struct` — Content, ToolCalls, Done, Usage, Error

### 5.2 Provider Registry

- [x] `NewRegistry()`
- [x] `Register(Provider) error` — errors on duplicate
- [x] `Get(id) (Provider, error)` — errors if not found
- [x] `List() []Provider`
- [x] `Remove(id)`
- [x] Test: register and retrieve
- [x] Test: duplicate registration errors
- [x] Test: get nonexistent errors

### 5.3 Ollama Provider

- [x] Default endpoint: `http://localhost:11434`
- [x] Health: `GET /api/tags`
- [x] `ListModels`: `GET /api/tags`
- [x] `Chat`: `POST /api/chat`
- [x] `ChatStream`: `POST /api/chat` stream=true, SSE parsing
- [x] Model pulling: `POST /api/pull` with progress
- [x] Embeddings: `POST /api/embeddings`
- [x] Error: connection refused → "Ollama not running"
- [x] Error: model not found → "Pull model from model manager"
- [x] Test: Chat returns expected response
- [x] Test: ChatStream emits deltas

### 5.4 OpenAI Provider

- [x] Endpoint: `https://api.openai.com/v1`
- [x] Auth: Bearer token
- [x] Models: GPT-4o, GPT-4o-mini, o1, o3-mini
- [x] `Chat`: `POST /v1/chat/completions`
- [x] `ChatStream` with `stream: true`, SSE `data:` lines
- [x] Tool calling (native OpenAI format)
- [ ] Structured output (`response_format`)
- [ ] Vision (image_url in messages)
- [x] Token counting via `tiktoken-go`
- [x] Cost tracking via lookup table
- [x] Rate limit backoff on 429
- [x] Error: 401 → "Invalid API key"
- [x] Error: 429 → "Rate limited. Retry in X seconds"
- [x] Test: Chat returns response
- [x] Test: ChatStream emits SSE
- [x] Test: tool calling returns tool_calls

### 5.5 Anthropic Provider

- [ ] Endpoint: `https://api.anthropic.com/v1`
- [ ] Auth: `x-api-key` header
- [ ] Models: Claude 3.5 Sonnet, Claude 4 Opus
- [ ] Messages API format
- [ ] Content blocks (text, tool_use, tool_result)
- [ ] Extended thinking mode
- [ ] SSE events: message_start, content_block_delta, message_stop
- [ ] Map content_block_delta → StreamDelta
- [ ] Test: Chat returns Claude response
- [ ] Test: tool_use blocks parsed

### 5.6 Gemini Provider

- [ ] Endpoint: `https://generativelanguage.googleapis.com/v1beta`
- [ ] Auth: API key as query parameter
- [ ] Models: Gemini 2.5 Pro, Gemini 2.0 Flash
- [ ] `generateContent` endpoint
- [ ] `streamGenerateContent` with SSE
- [ ] Safety settings configurable
- [ ] Test: generateContent returns response

### 5.7 Mistral Provider

- [ ] Endpoint: `https://api.mistral.ai/v1`
- [ ] Auth: Bearer token
- [ ] OpenAI-compatible API
- [ ] Test: Chat returns response

### 5.8 Groq Provider

- [ ] Endpoint: `https://api.groq.com/openai/v1`
- [ ] Auth: Bearer token
- [ ] OpenAI-compatible API
- [ ] Test: Chat returns response

### 5.9 Generic OpenAI-Compatible Provider

- [ ] Configurable endpoint URL
- [ ] Configurable API key
- [ ] Configurable custom headers
- [ ] Auto-detect models via `GET /v1/models`
- [ ] Test: works with vLLM-compatible API
- [ ] Test: custom headers injected

### 5.10 llama.cpp Provider

- [ ] Default endpoint: `http://localhost:8080`
- [ ] OpenAI-compatible API
- [ ] Works with llama-server, vLLM, LocalAI
- [ ] Test: Chat returns response

### 5.11 Multi-Model Router

- [x] `Router` struct with providers + rules
- [x] Routing rules: intent, content, tool
- [x] Default: chat→local, code→cloud, reasoning→cloud
- [x] Intent detection: analyze user message keywords
- [x] Explicit agent model bypasses routing
- [x] Per-message model override
- [x] Priority ordering
- [x] Test: route by intent
- [x] Test: explicit config bypasses routing
- [x] Test: no match → default

### 5.12 Model Fallback Chains

- [x] Configurable per agent
- [x] Try first, on failure try next
- [x] Triggers: 5xx, timeout, rate limit, connection failure
- [x] All fail → return last error
- [x] Each fallback logged in audit log
- [x] Test: fallback on 5xx
- [x] Test: fallback on timeout
- [x] Test: all fail returns error

### 5.13 Streaming

- [x] SSE parsing for all providers
- [x] Events via Wails `EventsEmit`
- [x] Types: token, tool_call_start, tool_call_end, done, error
- [x] WebSocket support for API clients
- [x] Svelte subscribes to stream events
- [x] Test: stream emits token events
- [x] Test: stream emits done with usage

### 5.14 Token Usage Tracking

- [x] Per-message: tokens_in, tokens_out
- [x] Per-session: cumulatives
- [x] Per-provider cost lookup table
- [x] Cost=0 for local providers
- [ ] Dashboard: daily/weekly/monthly charts
- [ ] Test: token counting accurate within 5%
- [ ] Test: cost matches pricing table

### 5.15 Context Window Management

- [x] Strategies: sliding_window, summarization, truncation
- [x] Sliding window: keep last N (default 50)
- [x] Always keep system prompt + latest 2
- [x] Summarization: model summarizes older messages
- [x] Truncation: hard truncate at max context
- [x] Warning at 80% (yellow) and 95% (red)
- [x] Test: sliding window drops oldest correctly
- [x] Test: summarization produces coherent summary
- [x] Test: truncation preserves newest

### 5.16 Safety-Aware Model Router

- [x] Extends base router with safety-awareness
- [x] Pre-route classification: analyze query for safety-sensitive content
- [ ] Classification categories: safe, security, code, creative, sensitive, unknown
- [ ] Query classification via lightweight model or rule-based detector
- [x] Detected "security" topic → prefer uncensored/abliterated model
- [x] Detected "code" topic → prefer code-specialized model
- [x] Detected "creative" → prefer high-temperature model
- [x] Routing decisions overridable by user preference
- [x] Per-agent safety routing config
- [x] Test: security query routes to uncensored model
- [x] Test: code query routes to code model

### 5.17 Refusal Detection & Response Classification

- [ ] Detect when model refuses to answer
- [ ] Refusal patterns: "I cannot", "I'm not able to", "As an AI", "I apologize", "I'm designed", "I cannot assist with", "I'm not allowed"
- [ ] Regex detection of refusal patterns in model output (first 200 chars)
- [ ] Model-assisted detection: ask a separate small model "Did the previous model refuse to answer?"
- [ ] Confidence scoring: 0.0-1.0 how certain the refusal detection is
- [ ] Threshold: confidence > 0.7 → treat as refusal
- [ ] On refusal detected → trigger fallback to next model in chain
- [ ] Log refusal events with source model, detected pattern, confidence
- [ ] Counter: track refusal rate per model over time
- [ ] High refusal rate (>50% over 10 queries) → auto-downgrade model priority
- [ ] Test: "tell me about security" triggers refusal on Gemma 4
- [ ] Test: false positive does not trigger (normal response passes)
- [ ] Test: refusal counter tracks correctly

### 5.18 Abliterated / Uncensored Model Support

- [ ] Provider type: `uncensored` — any provider can tag models as uncensored
- [ ] Model tags in provider config: `["safe"]`, `["uncensored"]`, `["abliterated"]`, `["code"]`
- [ ] Uncensored model list in config: explicit allowlist per provider
- [ ] Examples: `gemma-4-heretic`, `qwen3.6-abliterated`, `llama3.3-abliterated`, `dolphin-mixtral`
- [ ] Uncensored models flagged in UI with warning icon
- [ ] First use of uncensored model: warning dialog "This model has no safety filters"
- [ ] User must explicitly enable uncensored models in settings
- [ ] Per-agent toggle: "Allow uncensored fallback"
- [ ] Enabled: when safe model refuses → auto-fallback to uncensored
- [ ] Disabled: when safe model refuses → return refusal message to user
- [ ] Global off-switch: "Use uncensored models" in privacy settings
- [ ] Audit log marks all uncensored model calls
- [ ] Test: uncensored model enabled, safe model refuses → fallback fires
- [ ] Test: uncensored model disabled → refusal returned to user

### 5.19 Query Preservation During Model Switch

- [ ] When switching models, full context must be preserved
- [ ] Context includes: system prompt, all messages, tool results, memory context, user profile
- [ ] Model switch is transparent to conversation loop
- [ ] Switch happens at message level: retry the same message with different model
- [ ] Partial response from first model is discarded
- [ ] User sees: "Model {name} refused. Switching to {fallback}..."
- [ ] No messages lost during switch
- [ ] Token budget recalculated for new model
- [ ] Context window re-encoded for new model's tokenizer
- [ ] Streaming: interrupted on first model, restarted on fallback
- [ ] Max cascading switches: 3 (to prevent infinite loops)
- [ ] Test: context preserved exactly across model switch
- [ ] Test: streaming interrupted and restarted cleanly
- [ ] Test: max cascading switches enforced

### 5.20 Safety Level Configuration

- [ ] Per-agent safety level: safe | normal | relaxed | uncensored
- [ ] `safe` — only use models with strong safety guardrails, refuse on any uncertainty
- [ ] `normal` — default, try safe model first, fallback to uncensored on refusal
- [ ] `relaxed` — try uncensored first, no refusal detection needed
- [ ] `uncensored` — always use uncensored models, no filtering
- [ ] Safety level selector in agent config panel
- [ ] Config field: `model.safety_level` with env var `NALA_MODEL_SAFETY_LEVEL`
- [ ] Safety level shown in chat status bar
- [ ] Changing level mid-session applies to next message
- [ ] Test: safe level never falls back to uncensored
- [ ] Test: normal level falls back on refusal
- [ ] Test: relaxed level skips safe model entirely

---

## 6. Agent Runtime

### 6.1 Agent Manager

- [ ] `Manager.Create()` validates + inserts
- [ ] `Manager.Get()` by ID
- [ ] `Manager.GetBySlug()` by slug
- [ ] `Manager.Update()` validates + updates
- [ ] `Manager.Delete()` cascade to sessions
- [ ] `Manager.List()` with filters
- [ ] Slug auto-generated from name
- [ ] Slug manually overrideable
- [ ] Test: CRUD operations
- [ ] Test: duplicate slug (auto-append number)

### 6.2 Session Lifecycle

- [ ] `CreateSession()` creates + inserts
- [ ] `GetSession()` retrieves
- [ ] `ListSessions()` filterable
- [ ] `PauseSession()` save state, cancel goroutine
- [ ] `ResumeSession()` restore state, start goroutine
- [ ] `DeleteSession()` cascade delete
- [ ] Session TTL: paused 7d, completed 30d, error 7d
- [ ] Expiry check: startup + every hour
- [ ] Auto-title from first user message
- [ ] Test: create → pause → resume
- [ ] Test: session expiry

### 6.3 System Prompt Templating

- [ ] `{{date}}` — current date
- [ ] `{{time}}` — current time
- [ ] `{{datetime}}` — full datetime with TZ
- [ ] `{{user_name}}` — from settings
- [ ] `{{tools}}` — auto-generated tool catalog
- [ ] `{{tool_count}}` — number of tools
- [ ] `{{knowledge_bases}}` — KB names
- [ ] `{{session_id}}` — current session
- [ ] `{{agent_name}}` — agent's name
- [ ] Custom variables from metadata
- [ ] Unknown → left as-is
- [ ] Test: all built-in variables replaced
- [ ] Test: custom variables replaced

### 6.4 Personality Presets

- [ ] `default`: "You are a helpful AI assistant."
- [ ] `helpful`: "Be concise but thorough."
- [ ] `creative`: "Be imaginative and expressive."
- [ ] `precise`: "Be precise and factual."
- [ ] `code`: "Expert software engineer."
- [ ] `socratic`: "Guide user through questions."
- [ ] `custom`: user's prompt only
- [ ] Custom presets saved in `presets.json`
- [ ] Import/export presets
- [ ] Test: each preset produces correct prompt
- [ ] Test: unknown preset falls back to default

### 6.5 Conversation Loop

- [ ] Append user message to context
- [ ] Build full context (system + memory + history)
- [ ] Call model with context + tools
- [ ] No tool_calls → return text
- [ ] Tool_calls → execute each, append results, loop
- [ ] Max iterations: 20
- [ ] Max parallel tool calls: 10
- [ ] Empty response → retry once
- [ ] Repeated tool calls → inject note to stop
- [ ] Infinite loop detection → force text
- [ ] Token budget exceeded → inform user
- [ ] Test: simple chat works
- [ ] Test: single tool call works
- [ ] Test: multiple sequential tool calls
- [ ] Test: parallel tool calls
- [ ] Test: max iterations enforced
- [ ] Test: infinite loop detection

### 6.6 Tool Calling

- [ ] Parse tool_calls from model response
- [ ] Lookup tool in registry
- [ ] Tool not found → error to model
- [ ] Tool disabled → error to model
- [ ] Validate arguments against schema
- [ ] Invalid args → validation error to model
- [ ] Check permissions (require_approval?)
- [ ] If approval → pause, show UI, wait
- [ ] Execute with context timeout
- [ ] Return result as tool role
- [ ] Result > 100KB → truncated flag
- [ ] Test: valid tool call → executes
- [ ] Test: unknown tool → error returned
- [ ] Test: invalid args → validation error
- [ ] Test: tool timeout → timeout error

### 6.7 Structured Output

- [ ] Native JSON mode (GPT-4o: response_format)
- [ ] Prompt-based JSON mode (local models)
- [ ] JSON Schema validation
- [ ] Invalid → retry with error message
- [ ] Max retries: 3
- [ ] Exhausted → return error to caller
- [ ] Test: native JSON returns valid JSON
- [ ] Test: prompt-based works
- [ ] Test: invalid triggers retry

### 6.8 Session Persistence

- [ ] Save after every user message (sync)
- [ ] Save after every assistant message (sync)
- [ ] Save after every tool result (sync)
- [ ] Save on pause, complete
- [ ] Flush all on graceful shutdown
- [ ] Load full session on resume
- [ ] Load metadata only for list
- [ ] Test: session survives restart
- [ ] Test: messages persisted after each turn

### 6.9 Agent Concurrency

- [ ] Each session = goroutine
- [ ] Per-session context with cancel
- [ ] Model router: semaphore pool (max 4)
- [ ] Tool executor: semaphore pool (max 8)
- [ ] Max concurrent sessions: 10
- [ ] Over limit → "Too many active sessions"
- [ ] UI: status dots per session
- [ ] Test: multiple concurrent sessions
- [ ] Test: concurrency limit enforced

### 6.10 Agent Timeouts & Rate Limiting

- [ ] Per-request timeout (default 300s)
- [ ] Idle timeout (30 min → auto-pause)
- [ ] Session duration limit (24h → auto-complete)
- [ ] Per-agent rate limit (30 msgs/min)
- [ ] Per-provider rate limit
- [ ] Sliding window rate limiter
- [ ] Test: timeout kills stuck calls
- [ ] Test: idle timeout auto-pauses
- [ ] Test: rate limit blocks excess

---

## 7. Tool System

### 7.1 Tool Interface

- [ ] `Tool.ID() string`
- [ ] `Tool.Name() string`
- [ ] `Tool.Description() string`
- [ ] `Tool.ParameterSchema() json.RawMessage`
- [ ] `Tool.Execute(ctx, args) (*Result, error)`
- [ ] `Tool.ValidateArgs(args) error`
- [ ] `type Result struct` — Success, Content, MimeType, Data, DurationMs, Truncated

### 7.2 Tool Registry

- [ ] `Register(Tool) error`
- [ ] `RegisterMany(...Tool) error`
- [ ] `Get(id) (Tool, error)`
- [ ] `List(category) []Info`
- [ ] `All() []Info`
- [ ] `Categories() []string`
- [ ] `Unregister(id)`
- [ ] Categories: web, file, code, shell, db, http, image, memory, calendar, email, notes, agent, aur, system
- [ ] All built-in tools registered on startup
- [ ] Test: register and retrieve
- [ ] Test: duplicate errors

### 7.3 web.search

- [ ] ID: `web.search`
- [ ] Backends: DuckDuckGo (default), SearXNG, Bing, Google CSE, Tavily
- [ ] Args: query (required), max_results (1-20), region, safe_search
- [ ] Returns: [{title, url, snippet}]
- [ ] Markdown formatted
- [ ] Timeout: 10s
- [ ] Cache: TTL 5 min, LRU 100
- [ ] Error: rate limited → cached
- [ ] Error: no results → "No results found"
- [ ] Error: network → "Web search unavailable"
- [ ] Test: DuckDuckGo returns results
- [ ] Test: max_results respected

### 7.4 web.fetch

- [ ] ID: `web.fetch`
- [ ] Args: url (required), max_chars (default 10000), extract_mode (markdown|text|html)
- [ ] HTML→MD via go-readability
- [ ] Respects robots.txt
- [ ] User-Agent: `Nala/1.0`
- [ ] Cache: TTL 5 min, LRU 100
- [ ] Rate limit: 10/min per domain
- [ ] Timeout: 15s
- [ ] Test: URL fetched as markdown
- [ ] Test: max_chars truncation

### 7.5 file.read

- [ ] ID: `file.read`
- [ ] Args: path (required, relative to sandbox)
- [ ] Max size: 10MB
- [ ] Path traversal blocked
- [ ] Symlinks must resolve within sandbox
- [ ] Error: not found
- [ ] Error: outside sandbox
- [ ] Error: binary file
- [ ] Test: read returns content
- [ ] Test: traversal blocked

### 7.6 file.write

- [ ] ID: `file.write`
- [ ] Args: path, content, mode (overwrite|append|create_new)
- [ ] Parent dirs auto-created
- [ ] Max size: 10MB
- [ ] Error: file exists (create_new mode)
- [ ] Error: outside sandbox
- [ ] Test: write creates file
- [ ] Test: append adds to file

### 7.7 file.list

- [ ] ID: `file.list`
- [ ] Args: path, pattern (glob), recursive (bool)
- [ ] Returns: [{name, path, size, modified_at, is_dir}]
- [ ] Max entries: 10000
- [ ] Test: list returns files
- [ ] Test: pattern filters

### 7.8 file.delete

- [ ] ID: `file.delete`
- [ ] Args: path, permanent (bool, default false)
- [ ] permanent=false → moves to trash
- [ ] permanent=true → immediate (requires approval)
- [ ] Test: delete moves to trash
- [ ] Test: permanent deletes

### 7.9 code.execute

- [ ] ID: `code.execute`
- [ ] Args: language (python|js|go|bash), code, timeout_ms (default 30000), packages
- [ ] Python (venv subprocess)
- [ ] JavaScript (node subprocess)
- [ ] Go (go run subprocess)
- [ ] Bash (subprocess)
- [ ] Timeout, memory limit (256MB), no network, read-only FS
- [ ] Approval required for: Bash
- [ ] Error: timeout
- [ ] Error: memory exceeded
- [ ] Error: language not allowed
- [ ] Error: airgap (disabled)
- [ ] Test: Python prints hello world
- [ ] Test: timeout kills long code
- [ ] Test: network blocked

### 7.10 shell.run

- [ ] ID: `shell.run`
- [ ] Args: command (whitelisted), args (string[])
- [ ] Whitelist: ls, cat, head, tail, wc, find, grep, ps, df, du, uname, whoami, date, echo, pwd
- [ ] Conditional: ping, curl, wget, dig (require network)
- [ ] No chaining (;, &&, ||, |)
- [ ] No subshells ($(), backticks)
- [ ] Always requires approval
- [ ] Timeout: 15s
- [ ] Test: whitelisted command runs
- [ ] Test: non-whitelisted rejected
- [ ] Test: chaining blocked

### 7.11 db.query

- [ ] ID: `db.query`
- [ ] Args: query (SQL), params (optional map)
- [ ] Read-only: SELECT, PRAGMA, EXPLAIN only
- [ ] Write blocked: INSERT, UPDATE, DELETE, DROP, ALTER, CREATE
- [ ] Max rows: 1000
- [ ] Max length: 10000 chars
- [ ] Timeout: 30s
- [ ] Returns: {columns, rows, row_count}
- [ ] Test: SELECT returns results
- [ ] Test: INSERT blocked

### 7.12 http.request

- [ ] ID: `http.request`
- [ ] Args: method, url, headers (optional), body (optional), timeout_ms
- [ ] Methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
- [ ] Token replacement: {{api_key}}
- [ ] Redirects: max 5
- [ ] Response limit: 5MB
- [ ] Binary: base64-encoded
- [ ] Respects airgap mode
- [ ] Blocks localhost/internal IPs
- [ ] Test: GET returns response
- [ ] Test: airgap blocks

### 7.13 image.generate

- [ ] ID: `image.generate`
- [ ] Args: prompt, negative_prompt, width, height, steps, count (1), model
- [ ] Backends: SD WebUI, DALL-E 3, Imagen
- [ ] Stored in `{data_dir}/generated/`
- [ ] Inline preview in chat
- [ ] Test: image generation works

### 7.14 image.analyze

- [ ] ID: `image.analyze`
- [ ] Args: url, prompt
- [ ] Vision models only
- [ ] Non-vision → error
- [ ] Formats: JPEG, PNG, WebP, GIF
- [ ] Max size: 20MB
- [ ] Test: vision model analyzes

### 7.15 knowledge.search

- [ ] ID: `knowledge.search`
- [ ] Args: query, knowledge_base_id, top_k (5), min_score (0.7)
- [ ] Embed query → vector search → format results
- [ ] Test: search returns relevant results

### 7.16 memory.store / memory.recall

- [ ] `memory.store`: fact, category, importance
- [ ] `memory.recall`: query, top_k
- [ ] Test: store and recall

### 7.17 calendar.* (list, create, remind)

- [ ] Local iCal storage
- [ ] Args: title, start_time, duration_minutes, description, reminder
- [ ] Test: create event

### 7.18 email.* (send, inbox, read)

- [ ] SMTP sending (encrypted creds)
- [ ] IMAP inbox listing
- [ ] Test: send email

### 7.19 notes.* (create, list, read, update, delete)

- [ ] `{data_dir}/notes/` as .md files
- [ ] YAML frontmatter: title, tags, created, modified
- [ ] Test: create and read note

### 7.20 Custom Tool SDK

- [ ] Manifest: `manifest.yaml` (id, name, description, language, entrypoint, permissions)
- [ ] Lua via `gopher-lua`
- [ ] Python via JSON-RPC subprocess
- [ ] Manifest permissions enforced
- [ ] Test: Lua tool executes
- [ ] Test: Python tool executes

### 7.21 Tool Sandboxing

- [ ] Sandbox root: `{data_dir}/sandbox/`
- [ ] Path traversal blocked
- [ ] Linux: `unshare(CLONE_NEWNET)` network isolation
- [ ] RLIMIT_AS (256MB), RLIMIT_CPU, RLIMIT_FSIZE (10MB)
- [ ] seccomp: block dangerous syscalls
- [ ] Subprocess as nobody (uid 65534)
- [ ] Fallback: subprocess + rlimits only
- [ ] Test: prevents FS escape
- [ ] Test: prevents network

### 7.22 Tool Permission System

- [ ] Levels: allow, approve_once, approve_session, deny
- [ ] Defaults: safe→allow, sensitive→approve_once, dangerous→approve_session
- [ ] Agent override per tool
- [ ] Approval modal: tool + args + approve/deny/session buttons
- [ ] Session paused while waiting
- [ ] Timeout: 5 min → auto-deny
- [ ] Audit logged
- [ ] Test: each level behaves correctly

### 7.23 aur.search

- [ ] ID: `aur.search`
- [ ] Args: query (required), limit (1-50, default 10), by (name|desc|maintainer)
- [ ] Searches Arch User Repository for packages
- [ ] Returns: [{name, version, description, votes, popularity, maintainer, last_modified}]
- [ ] Filters: out-of-date, orphaned
- [ ] Timeout: 10s
- [ ] Error: AUR unreachable → "AUR search unavailable"
- [ ] Error: no results → "No packages found"
- [ ] Test: search returns results
- [ ] Test: limit respected

### 7.24 aur.info

- [ ] ID: `aur.info`
- [ ] Args: package (required)
- [ ] Returns: full package info (name, version, description, url, license, depends, makedepends, conflicts, provides, votes, popularity, maintainer, last_modified, out_of_date, keywords, package_base_id)
- [ ] Timeout: 10s
- [ ] Error: package not found → "Package not found"
- [ ] Test: info returns valid data

### 7.25 aur.install

- [ ] ID: `aur.install`
- [ ] Args: packages (string[], required), asdeps (bool, default false)
- [ ] Installs AUR packages via yay/paru (auto-detect helper)
- [ ] Requires approval (always)
- [ ] Streams output to chat
- [ ] Timeout: 300s (5 minutes)
- [ ] Error: helper not found → "Install yay or paru first"
- [ ] Error: package conflicts → show conflict list
- [ ] Test: install with mock helper

### 7.26 aur.remove

- [ ] ID: `aur.remove`
- [ ] Args: packages (string[], required), cascade (bool, default false)
- [ ] Removes AUR packages via pacman
- [ ] Requires approval (always)
- [ ] Streams output to chat
- [ ] Timeout: 120s
- [ ] Error: package not installed → warn
- [ ] Error: dependency of other package → list dependents
- [ ] Test: remove with mock

### 7.27 aur.update

- [ ] ID: `aur.update`
- [ ] Args: packages (string[], optional — all if empty)
- [ ] Updates AUR packages via yay/paru
- [ ] Requires approval (always)
- [ ] Streams output to chat
- [ ] Timeout: 300s
- [ ] Shows: {package, old_version, new_version} before approval
- [ ] Error: no updates → "All packages up to date"
- [ ] Test: update with mock

### 7.28 aur.list

- [ ] ID: `aur.list`
- [ ] Args: filter (optional string), upgradable_only (bool, default false)
- [ ] Lists installed AUR packages
- [ ] Returns: [{name, version, description, installed_size}]
- [ ] No approval required (read-only)
- [ ] Test: list returns packages

### 7.29 system.monitor

- [ ] ID: `system.monitor`
- [ ] Args: metric (cpu|memory|disk|network|all, default all), duration (optional, historical)
- [ ] Returns current system metrics
- [ ] CPU: per-core + aggregate usage, load average
- [ ] Memory: total, used, available, swap
- [ ] Disk: per-mount usage, inodes
- [ ] Network: per-interface bytes/packets in/out
- [ ] Duration: last_5m, last_1h, last_6h, last_24h (historical from ring buffer)
- [ ] Timeout: 5s
- [ ] Test: returns CPU data
- [ ] Test: returns memory data

### 7.30 system.processes

- [ ] ID: `system.processes`
- [ ] Args: sort_by (cpu|memory|pid|name, default cpu), order (asc|desc, default desc), filter (optional string), limit (1-100, default 20)
- [ ] Lists running processes
- [ ] Returns: [{pid, name, cpu_percent, memory_percent, memory_rss, state, user, cmdline}]
- [ ] Timeout: 5s
- [ ] Test: returns process list

### 7.31 system.logs

- [ ] ID: `system.logs`
- [ ] Args: query (optional), level (debug|info|warn|error, optional), limit (1-1000, default 100), since (ISO 8601, optional)
- [ ] Returns recent Nala log entries
- [ ] Returns: [{timestamp, level, message, fields}]
- [ ] Timeout: 5s
- [ ] Test: returns log entries

### 7.32 system.notify

- [ ] ID: `system.notify`
- [ ] Args: title (required), message (required), urgency (low|normal|critical, default normal), timeout_ms (optional)
- [ ] Sends desktop notification via libnotify
- [ ] Always requires approval
- [ ] Timeout: 5s
- [ ] Test: notification sent

### 7.33 file.search

- [ ] ID: `file.search`
- [ ] Args: pattern (glob or regex, required), root (optional, default sandbox), recursive (bool, default true), max_results (1-1000, default 100), type (file|dir|all, default all)
- [ ] Searches files by pattern
- [ ] Path traversal blocked (relative to root)
- [ ] Respects sandbox boundaries
- [ ] Symlinks: follow or skip (configurable, default skip)
- [ ] Binary files: listed but content not searched
- [ ] Hidden files: include or exclude (default exclude)
- [ ] Returns: [{name, path, size, modified_at, is_dir, is_symlink}]
- [ ] Timeout: 30s
- [ ] Error: pattern too broad → "Pattern matches too many files, refine"
- [ ] Test: search by glob pattern
- [ ] Test: traversal blocked
- [ ] Test: max_results respected

---

## 8. Memory & Knowledge

### 8.1 Vector Database

- [ ] Backend: sqlite-vec (MVP)
- [ ] `Insert(collection, id, vector, metadata)`
- [ ] `Search(collection, vector, topK) → []Result`
- [ ] `Delete(collection, id)`
- [ ] `CollectionStats(collection)`
- [ ] Cosine similarity
- [ ] Collections: documents, conversations, user_memory
- [ ] Test: insert and search
- [ ] Test: delete removes

### 8.2 Document Ingestion

- [ ] Formats: PDF, TXT, MD, CSV, JSON, HTML, DOCX
- [ ] Pipeline: detect MIME → extract → chunk → embed → store
- [ ] PDF via `ledongthuc/pdf`
- [ ] HTML via `goquery` → markdown
- [ ] DOCX via `unioffice`
- [ ] Duplicate detection via SHA-256
- [ ] Progress reporting via Wails events
- [ ] Binary files: metadata only
- [ ] Encrypted files: error
- [ ] Large files >100MB: background
- [ ] Test: PDF text extraction

### 8.3 Document Chunking

- [ ] Fixed-size: N chars, M overlap
- [ ] Recursive: paragraph → sentence → word
- [ ] Default: 1000 chars, 200 overlap
- [ ] Chunk metadata: {index, content, token_count}
- [ ] Test: recursive respects boundaries

### 8.4 Embedding Providers

- [ ] Interface: `Embed(ctx, texts) ([][]float32, error)`
- [ ] Ollama: nomic-embed-text (768d)
- [ ] OpenAI: text-embedding-3-small (1536d)
- [ ] Cache: content hash → vector
- [ ] Batch: 10 texts per call
- [ ] Test: returns correct dimensions

### 8.5 RAG Pipeline

- [ ] Embed query, vector search, filter by min_score
- [ ] Format: `Source: filename (score: X)\ncontent`
- [ ] Inject into system prompt
- [ ] Test: returns relevant context
- [ ] Test: min_score filters

### 8.6 Knowledge Base CRUD

- [ ] Create KB with name, embedding model, strategy
- [ ] List, Get, Update, Delete
- [ ] Cascade documents on delete
- [ ] Test: CRUD

### 8.7 Conversation Memory

- [ ] Short-term: full message history
- [ ] Managed by context window strategy
- [ ] Long-term: periodic summarization
- [ ] Summarization model configurable
- [ ] Summaries stored as vectors
- [ ] Retrieved when relevant
- [ ] Test: short-term preserves context

### 8.8 User Memory

- [ ] Auto-extraction every N messages (default 5)
- [ ] Prompt: "Extract facts about the user"
- [ ] Categories: preference, fact, relationship, schedule
- [ ] Dedup by cosine similarity > 0.95
- [ ] Manual store via memory.store
- [ ] UI: "Remember this" button
- [ ] Recall on session start
- [ ] Settings: "What Nala knows about you"
- [ ] Test: facts auto-extracted
- [ ] Test: dedup works

### 8.9 Memory Consolidation

- [ ] Runs every 60 minutes
- [ ] Sessions with > 50 msgs and idle > 1h
- [ ] Summarize messages older than last 20
- [ ] Store as long-term memory
- [ ] Remove summarized messages
- [ ] Test: consolidation runs

### 8.10 Full-Text Search

- [ ] SQLite FTS5 on documents, messages, notes, user memory
- [ ] Combined search across all
- [ ] Results: {entity_type, entity_id, title, snippet, score}
- [ ] < 500ms at 100K docs
- [ ] Test: returns relevant results

### 8.11 Entity Extraction

- [ ] Extract entities from user messages and model responses
- [ ] Entity types: person, organization, project, event, date, location, email_address, phone, url, tool_name, code_language, technology, concept
- [ ] Extraction method: model-assisted (prompt-based for accuracy)
- [ ] Lightweight extraction: regex+NLP for immediate offline use
- [ ] Entities stored in `user_data` table with type, value, context, source_message_id, confidence
- [ ] Entity relationships: person→project, person→event, project→technology, etc.
- [ ] Relationship stored: {source_entity, relation, target_entity, context, confidence}
- [ ] Entity dedup: same type + same value → merge, update confidence
- [ ] Entity conflict: same entity, different values → keep both with lower confidence
- [ ] Entity extraction on every message (async, non-blocking)
- [ ] Batch extraction on session end for historical messages
- [ ] Test: "met Sarah at the C++ conference" extracts Person(Sarah), Event(C++ conference)
- [ ] Test: entities merged correctly on dedup
- [ ] Test: relationships extracted correctly

### 8.12 Cross-Source Memory

- [ ] Memory sources: conversations, emails, calendar events, notes, documents, user_data
- [ ] Unified entity index across all sources
- [ ] When user mentions "the project from that email" → find email → extract project name → search other sources
- [ ] When user mentions "the conference" → search calendar events + conversation history + emails
- [ ] Source linking: entity from conversation linked to same entity in email/calendar/note
- [ ] Link confidence: higher if entity name matches exactly, lower if fuzzy match
- [ ] Link rebuilding: periodic job to find and connect unlinked entities
- [ ] Cross-source search API: `FindEntity(value string, sources []SourceType) → []EntityResult`
- [ ] EntityResult: {entity, source_type, source_id, context_snippet, timestamp, confidence}
- [ ] Results merged across sources, deduplicated, sorted by confidence+recency
- [ ] Test: entity found in email and conversation returns both sources
- [ ] Test: fuzzy matching links "Sarah" from email to "Sara" in conversation

### 8.13 Temporal Reasoning

- [ ] Parse time expressions from user queries: "last week", "yesterday", "a few days ago", "on Tuesday", "last month", "earlier this year"
- [ ] Time expression parser: rule-based with common patterns
- [ ] Supported patterns:
  - [ ] "last {dayofweek}" → previous occurrence of that day
  - [ ] "this {week/month/year}" → current period
  - [ ] "last {week/month/year}" → previous period
  - [ ] "{N} {days/weeks/months} ago" → relative offset
  - [ ] "on {date}" → specific date parsing
  - [ ] "in {month}" → events in that month
  - [ ] "earlier/later this {period}" → relative within period
  - [ ] "around {time}" ± buffer window
- [ ] Resolved time range applied as filter to memory/entity search
- [ ] Fuzzy time: "recently" → last 7 days
- [ ] Temporal anchor: "before X" / "after X" relative to events
- [ ] Combine multiple temporal clauses: "last week at the conference"
- [ ] Test: "last week" resolves to correct date range
- [ ] Test: "the C++ conference last week" finds event in calendar + conversations
- [ ] Test: "the project she talked about" chains through person → event → project

### 8.14 Memory Confidence Scoring

- [ ] Every memory retrieval includes confidence score 0.0-1.0
- [ ] Confidence factors:
  - [ ] Recency: events within last day = 1.0, last week = 0.9, last month = 0.7, last year = 0.4
  - [ ] Source reliability: explicit(user said) = 1.0, inferred = 0.6, model-extracted = 0.7
  - [ ] Entity match quality: exact = 1.0, fuzzy >0.9 = 0.7, fuzzy >0.8 = 0.4
  - [ ] Cross-source confirmation: same entity in 2+ sources = +0.2
  - [ ] Temporal match: query time range matches event time = 1.0, outside = 0.3
- [ ] Combined confidence: weighted average of factors
- [ ] Thresholds: high (>0.8) → include directly, medium (0.5-0.8) → include with "maybe" qualifier, low (<0.5) → exclude unless no better results
- [ ] Confidence displayed in "What Nala knows about you" UI
- [ ] User can correct confidence: "Yes, that's right" → boost, "No, that's wrong" → demote
- [ ] Test: recent event scores higher than old event
- [ ] Test: cross-source confirmation boosts score

### 8.15 Multi-Hop Memory Retrieval

- [ ] Chain multiple retrievals to answer complex queries
- [ ] Query decomposition: break "who was that girl I met at the C++ conference last week and what was the project she was talking about?" into sub-queries:
  - [ ] Hop 1: Find "C++ conference" event in last week → get event_id
  - [ ] Hop 2: Find people associated with event_id → get person_id
  - [ ] Hop 3: Find "project" discussions involving person_id → get project_name
  - [ ] Hop 4: Retrieve full context about project_name
- [ ] Each hop uses entity index + vector search + temporal filter
- [ ] Results from each hop feed into next hop's query
- [ ] Max hops: 5 (prevent runaway chains)
- [ ] Hop metadata logged: {hop_number, query, result_count, confidence, duration_ms}
- [ ] If a hop returns zero results: backtrack, try alternative path
- [ ] Alternative paths: broader search (lower threshold), different entity type, larger time window
- [ ] Final result: synthesized from all hops with confidence and source citations
- [ ] Test: 2-hop query returns correct person+project
- [ ] Test: 4-hop query with backtracking succeeds
- [ ] Test: zero results at any hop → graceful "I don't know"
- [ ] THE CRITICAL FLOW: "who was that girl I met at the C++ conference last week and what was the project she was talking about?"
  - [ ] Step 1: Extract entities → Person("girl"), Event("C++ conference"), Time("last week"), Project("project")
  - [ ] Step 2: Temporal filter → TimeWindow = last 7 days
  - [ ] Step 3: Entity search → find Event matching "C++ conference" in time window
  - [ ] Step 4: From Event, find all Person entities associated (attendees, mentions)
  - [ ] Step 5: Filter Person by gender (if available in entity metadata)
  - [ ] Step 6: From Person, find all Project entities they discussed
  - [ ] Step 7: Retrieve full context for Project (conversation snippets, email subjects)
  - [ ] Step 8: Format response with confidence, sources, and citations
  - [ ] Edge case: multiple conferences in time window → ask user to clarify
  - [ ] Edge case: person found but no project mentioned → report what is known
  - [ ] Edge case: conference found but no people extracted → return partial result

### 8.16 Memory Query Interface

- [ ] Unified query API: `QueryMemory(ctx, Query) → MemoryResponse`
- [ ] Query struct: {text, entity_types[], time_range, sources[], max_hops, min_confidence}
- [ ] Auto-detection: parse natural language query to fill Query struct
- [ ] Response struct: {answer, confidence, hops[], sources[], raw_entities[], time_to_live_ms}
- [ ] Memory can be queried via:
  - [ ] Conversation loop: auto-injected before model call when relevant
  - [ ] Tool call: `memory.query` tool for agent-initiated retrieval
  - [ ] API: `POST /api/v1/memory/query` for external access
  - [ ] UI: search bar in "What Nala knows about you" page
- [ ] Memory query logging: all queries logged for debugging and improvement
- [ ] Memory query timeout: 10s max
- [ ] Test: "C++ conference" query returns correct multi-hop result
- [ ] Test: empty query returns recent memories

---

## 9. Multi-Agent & Orchestration

### 9.1 Agent Delegation (agent.delegate)

- [ ] Args: agent_id, task, context, async
- [ ] Sync: wait for response
- [ ] Async: return task ID
- [ ] All delegations logged
- [ ] Test: sync returns result
- [ ] Test: async returns task ID

### 9.2 Pipeline Engine

- [ ] Pipeline: name, description, steps[]
- [ ] Step: agent_id, input_template, output_key, depends_on, max_retries
- [ ] DAG execution: deps-first, parallel independent
- [ ] Variable sub: {{input.key}}, {{steps.step_id.key}}
- [ ] Error strategies: stop, continue, retry
- [ ] Status tracking: pending, running, completed, error, cancelled
- [ ] Test: sequential executes in order
- [ ] Test: parallel executes concurrently
- [ ] Test: variables substituted

### 9.3 Orchestrator Agent

- [ ] System prompt with agent discovery
- [ ] Has agent.delegate tool
- [ ] Breaks down tasks, delegates
- [ ] Synthesizes final response
- [ ] Shows plan before execution
- [ ] Test: delegates to specialist

### 9.4 Swarm Mode

- [ ] Config: agents[], task_template, merge_strategy, max_concurrent
- [ ] Strategies: concatenate, vote, best_of
- [ ] Test: parallel execution
- [ ] Test: concatenate merges

### 9.5 Agent-as-Tool

- [ ] Each agent = tool `agent.{slug}`
- [ ] Parameters: {task, context}
- [ ] Delegates to target agent
- [ ] Target's permissions apply
- [ ] Test: agent available as tool

### 9.6 Human-in-the-Loop

- [ ] Tool approval pauses session
- [ ] Agent can request input
- [ ] HITL modal: input field + approve/deny
- [ ] Timeout: 5 min → continue
- [ ] Audit logged
- [ ] Test: modal appears
- [ ] Test: timeout continues

### 9.7 Scheduled Agent Runs

- [ ] Cron via `robfig/cron/v3`
- [ ] 5-field cron expressions
- [ ] Timezone support
- [ ] Max runs limit (0 = unlimited)
- [ ] Catch-up on startup
- [ ] Persisted in SQLite
- [ ] Test: runs at correct time
- [ ] Test: max_runs stops

### 9.8 Event-Triggered Agents

- [ ] Types: file_change (fsnotify), webhook (HTTP), timer, startup
- [ ] Test: file change triggers
- [ ] Test: webhook triggers

---

## 10. User Interface

### 10.1 Chat Interface

- [ ] Nav bar: logo, agent selector, settings, search
- [ ] Sidebar: session list (new, items with status dot)
- [ ] Chat area: message list (virtualized)
- [ ] User msgs: right, assistant: left
- [ ] Markdown via `marked`
- [ ] Code highlighting via `highlight.js`
- [ ] Inline images
- [ ] Clickable links
- [ ] Input: auto-resizing textarea, Enter=send, Shift+Enter=newline
- [ ] Send button disabled while generating
- [ ] Attach button (file picker)
- [ ] Model override dropdown
- [ ] Status bar: sessions, model, tokens
- [ ] Loading: skeleton messages
- [ ] Empty: "Start a conversation..."
- [ ] Error: inline + retry

### 10.2 Streaming Token Display

- [ ] Character-by-character
- [ ] Blinking cursor during stream
- [ ] Auto-scroll with pause on manual
- [ ] States: idle, connecting, streaming, done, error

### 10.3 Tool Call Visualization

- [ ] Card: icon, name, status, duration, expandable
- [ ] States: pending (gray), running (blue), completed (green), error (red), denied (orange)

### 10.4 Agent Configuration Panel

- [ ] Identity: name, slug, description, personality
- [ ] System prompt with `{{` autocomplete
- [ ] Model: provider, model, parameters
- [ ] Tools: searchable list, enable/disable, permissions
- [ ] Memory: KB select, context strategy
- [ ] Advanced: timeout, retries, iterations, fallback
- [ ] Inline validation, save disabled until valid

### 10.5 Model Management UI

- [ ] Provider cards: icon, name, status, model count
- [ ] Add: type, endpoint, API key (masked), test connection
- [ ] Auto-detect models
- [ ] Edit/disable/delete

### 10.6 Tool Management UI

- [ ] Catalog grid by category
- [ ] Each: icon, name, description, enable/disable
- [ ] Expanded: params, permissions, config

### 10.7 Knowledge Base UI

- [ ] KB list: cards (name, doc count, model)
- [ ] Detail: document list with status
- [ ] Upload: multi-file, drag-drop, progress bar
- [ ] Query tester: input, results with scores

### 10.8 Pipeline Builder

- [ ] Agent palette → drag to canvas
- [ ] Connect steps (output → input dots)
- [ ] Step config on click
- [ ] Test run, export/import JSON

### 10.9 Session Browser

- [ ] Infinite scroll, columns (title, agent, status, date, msgs, tokens)
- [ ] Sort/filter
- [ ] Full-text search
- [ ] Detail: read-only messages
- [ ] Continue, Export (JSON/MD/txt), Delete

### 10.10 Log Viewer

- [ ] tail -f style, color-coded
- [ ] Filter by level, search, date range
- [ ] Auto-scroll with pause
- [ ] Copy line, Export (JSON/txt), Clear

### 10.11 Settings Page

- [ ] Tabs: General, Models, Tools, Privacy, System, About
- [ ] Auto-save with debounce
- [ ] "Restart required" badge

### 10.12 System Tray Menu

- [ ] Show Nala, New Chat, Recent Agents (5), Settings, Quit

### 10.13 Onboarding Wizard

- [ ] Steps: Welcome → Model Detection → Cloud (optional) → Default Agent → Done
- [ ] First run only

### 10.14 Agent Templates

- [ ] Bundled: Research Assistant, Code Reviewer, Writing Coach, Data Analyst
- [ ] One-click install

### 10.15 Token Usage Dashboard

- [ ] Daily bar chart (7 day), Monthly line chart (30 day)
- [ ] Cost pie chart (by model), Top agents bar chart
- [ ] Metrics: total tokens, cost, avg/session

---

## 11. Plugin System

### 11.1 Plugin Manifest

- [ ] `manifest.yaml`: id, name, version, description, author, license
- [ ] entrypoint: {type: wasm|lua|python, file}
- [ ] permissions: network, filesystem, capabilities
- [ ] tools: [{id, name, description, parameters}]
- [ ] lifecycle: on_load, on_unload

### 11.2 WASM Runtime

- [ ] Via `wazero` (pure Go)
- [ ] Host functions: log, http_get, file_read, file_write
- [ ] Permission-gated
- [ ] Memory: 64MB limit
- [ ] Test: WASM executes
- [ ] Test: memory limit

### 11.3 Lua Runtime

- [ ] Via `gopher-lua`
- [ ] API: nala.log, nala.http_get, nala.file_read, nala.file_write, nala.json
- [ ] Restricted env (no os, io, debug)
- [ ] Test: Lua executes

### 11.4 Python Runtime

- [ ] JSON-RPC subprocess protocol
- [ ] Auto-venv + requirements.txt
- [ ] Test: Python executes

### 11.5 Plugin Isolation

- [ ] WASM: sandboxed by design
- [ ] Lua: restricted env
- [ ] Python: subprocess limits
- [ ] Crash → restart, doesn't affect others
- [ ] Hang → kill after timeout

### 11.6 Plugin Auto-Update

- [ ] Check: startup + 24h
- [ ] Download, verify checksum, install
- [ ] Backup old version for rollback

---

## 12. API & Integration

### 12.1 REST API Endpoints

- [ ] `GET /api/v1/agents` — list agents
- [ ] `POST /api/v1/agents` — create agent
- [ ] `GET /api/v1/agents/:id` — get agent
- [ ] `PUT /api/v1/agents/:id` — update agent
- [ ] `DELETE /api/v1/agents/:id` — delete agent
- [ ] `GET /api/v1/agents/:id/tools` — list tool bindings
- [ ] `PUT /api/v1/agents/:id/tools` — update tool bindings
- [ ] `GET /api/v1/sessions` — list sessions
- [ ] `POST /api/v1/sessions` — create session
- [ ] `GET /api/v1/sessions/:id` — get session
- [ ] `PATCH /api/v1/sessions/:id` — update session
- [ ] `DELETE /api/v1/sessions/:id` — delete session
- [ ] `GET /api/v1/sessions/:id/messages` — list messages
- [ ] `POST /api/v1/sessions/:id/messages` — send message
- [ ] `GET /api/v1/sessions/:id/messages/:mid` — get message
- [ ] `GET /api/v1/sessions/:id/stream` — SSE stream
- [ ] `GET /api/v1/models` — list all models
- [ ] `GET /api/v1/providers` — list providers
- [ ] `POST /api/v1/providers` — add provider
- [ ] `PUT /api/v1/providers/:id` — update provider
- [ ] `DELETE /api/v1/providers/:id` — delete provider
- [ ] `POST /api/v1/providers/:id/test` — test connection
- [ ] `GET /api/v1/knowledge-bases` — list KBs
- [ ] `POST /api/v1/knowledge-bases` — create KB
- [ ] `GET /api/v1/knowledge-bases/:id` — get KB
- [ ] `DELETE /api/v1/knowledge-bases/:id` — delete KB
- [ ] `GET /api/v1/knowledge-bases/:id/documents` — list documents
- [ ] `POST /api/v1/knowledge-bases/:id/documents` — upload document
- [ ] `DELETE /api/v1/knowledge-bases/:id/documents/:did` — delete document
- [ ] `POST /api/v1/knowledge-bases/:id/search` — search KB
- [ ] `GET /api/v1/tools` — list tools
- [ ] `GET /api/v1/tools/categories` — list categories
- [ ] `GET /api/v1/pipelines` — list pipelines
- [ ] `POST /api/v1/pipelines` — create pipeline
- [ ] `GET /api/v1/pipelines/:id` — get pipeline
- [ ] `PUT /api/v1/pipelines/:id` — update pipeline
- [ ] `DELETE /api/v1/pipelines/:id` — delete pipeline
- [ ] `POST /api/v1/pipelines/:id/execute` — execute pipeline
- [ ] `GET /api/v1/pipelines/:id/runs/:runid` — run status
- [ ] `GET /api/v1/system/health` — health check
- [ ] `GET /api/v1/system/status` — system status
- [ ] `GET /api/v1/system/config` — get config
- [ ] `PUT /api/v1/system/config` — update config
- [ ] `GET /api/v1/system/logs` — get logs
- [ ] `POST /api/v1/system/backup` — create backup
- [ ] `POST /api/v1/system/restore` — restore backup
- [ ] `GET /api/v1/notes` — list notes (future)
- [ ] `POST /api/v1/notes` — create note (future)
- [ ] `GET /api/v1/notes/:id` — get note (future)
- [ ] `PUT /api/v1/notes/:id` — update note (future)
- [ ] `DELETE /api/v1/notes/:id` — delete note (future)
- [ ] `GET /api/v1/notes/search` — search notes (future)
- [ ] `GET /api/v1/calendar/events` — list events (future)
- [ ] `POST /api/v1/calendar/events` — create event (future)
- [ ] `GET /api/v1/calendar/events/:id` — get event (future)
- [ ] `PUT /api/v1/calendar/events/:id` — update event (future)
- [ ] `DELETE /api/v1/calendar/events/:id` — delete event (future)
- [ ] `GET /api/v1/email/accounts` — list email accounts (future)
- [ ] `POST /api/v1/email/accounts` — add account (future)
- [ ] `GET /api/v1/email/messages` — list messages (future)
- [ ] `POST /api/v1/email/send` — send email (future)
- [ ] `GET /api/v1/skills` — list skills (future)
- [ ] `POST /api/v1/skills/:id/install` — install skill (future)
- [ ] `POST /api/v1/skills/:id/toggle` — enable/disable skill (future)
- [ ] `GET /api/v1/system/monitor` — system metrics (future)
- [ ] `GET /api/v1/system/processes` — process list (future)
- [ ] `GET /api/v1/automation/triggers` — list triggers (future)
- [ ] `POST /api/v1/automation/triggers` — create trigger (future)
- [ ] `GET /api/v1/user/profile` — user profile (future)
- [ ] `PUT /api/v1/user/profile` — update profile (future)

### 12.2 API Auth

- [ ] localhost: no auth (default)
- [ ] Remote: X-API-Key header
- [ ] Key generated on first launch, bcrypt hashed
- [ ] Rate limit: 60 RPM per IP, 429 + Retry-After

### 12.3 Webhook

- [ ] `POST /api/v1/webhooks/{id}`
- [ ] Config: agent_id, secret, input_template
- [ ] Variables: `{{event.body}}`, `{{event.headers.X-Custom}}`
- [ ] Response: 202 + run_id

### 12.4 WebSocket

- [ ] `ws://localhost:8472/api/v1/ws`
- [ ] Events: session.*, stream:*, system.*, agent.*
- [ ] Client: subscribe, unsubscribe, ping
- [ ] Heartbeat: pong every 30s

### 12.5 CLI Client

- [ ] `nala chat` — interactive readline
- [ ] `nala chat --message "..."` — single message
- [ ] `nala run --agent X --input Y`
- [ ] `nala config get/set`
- [ ] `nala session list/get/delete`
- [ ] `nala model list`
- [ ] `nala provider list/add`
- [ ] `nala tool list`
- [ ] `nala kb list/search`
- [ ] `nala pipeline list/run`
- [ ] `nala system health/status/backup/restore`
- [ ] `nala plugin list/install`
- [ ] `nala note list/create/edit/search`
- [ ] `nala calendar list/create`
- [ ] `nala email send/inbox`
- [ ] `nala skill list/install`
- [ ] `nala aur search/info/list`
- [ ] `nala monitor status`
- [ ] `nala trigger list/create`
- [ ] `--format json|text|table`

---

## 13. Security & Privacy

### 13.1 API Key Encryption

- [ ] AES-256-GCM
- [ ] Key derived via HKDF-SHA256 from machine seed
- [ ] Machine seed: `~/.config/nala/.machine-key`
- [ ] Plaintext never logged or exposed
- [ ] Hint: last 4 chars only
- [ ] Test: encrypt/decrypt round-trip

### 13.2 Tool Sandboxing

- [ ] Linux: unshare(CLONE_NEWNET) network isolation
- [ ] Linux: seccomp filter
- [ ] RLIMIT: AS(256MB), CPU, FSIZE(10MB)
- [ ] Subprocess: nobody user (uid 65534)
- [ ] Fallback: subprocess + rlimits
- [ ] Test: FS escape blocked
- [ ] Test: network blocked

### 13.3 Permission System

- [ ] Levels: allow, approve_once, approve_session, deny
- [ ] Default: safe→allow, sensitive→approve_once, dangerous→approve_session
- [ ] Agent override
- [ ] Approval modal with timeout (5min)
- [ ] Audit logged

### 13.4 Audit Logging

- [ ] Categories: tool_exec, model_call, agent_action, config_change, auth, system
- [ ] Retention: configurable (default 90 days)
- [ ] Auto-prune on startup
- [ ] UI: filterable, exportable

### 13.5 Airgap Mode

- [ ] Blocks all external network
- [ ] Local models still work
- [ ] Disables: web.search, web.fetch, http.request, email, plugin updates
- [ ] UI shield indicator

### 13.6 Incognito Mode

- [ ] Per-session toggle
- [ ] No persistence, no audit, no history
- [ ] Auto-deleted on close
- [ ] Warning shown

### 13.7 Data Export

- [ ] ZIP: agents.json, sessions.json, messages.json, KBs, documents, notes, config
- [ ] API keys NOT exported
- [ ] Test: valid ZIP created

### 13.8 Data Purge (Complete User Data Deletion)

- [ ] Settings → Purge All Data (irreversible option)
- [ ] Type "PURGE" to confirm (prevents accidental clicks)
- [ ] Deletes ALL user data: conversations, memory, emails, calendar events, notes, skills, plugins, settings, themes, logs, cache, preferences, model configurations, API keys, personalization data, entity graph, embeddings, vector indices, FTS indexes, session data, agent configs, stored credentials
- [ ] Database: DROP ALL tables, VACUUM, shrink file to minimum
- [ ] Files: delete all created files in data directory (notes, exports, backups, cached models)
- [ ] Config: reset `~/.config/nala/config.toml` to defaults
- [ ] Logs: clear all log files
- [ ] Auto-backup before purge: create timestamped backup archive in ~/.local/share/nala/backups/
- [ ] Backup path: `~/.local/share/nala/backups/purge-{timestamp}.tar.zst`
- [ ] Backup notification: "Backup saved to {path}. You have 7 days to restore before auto-deletion."
- [ ] Confirmation dialog shows: backup path, what will be deleted, WARNING: IRREVERSIBLE
- [ ] Secondary confirmation: type "I UNDERSTAND THIS DELETES ALL MY DATA" (not just PURGE)
- [ ] Purge progress bar in UI
- [ ] After purge: application restart required
- [ ] Restart notification: "Nala must restart to complete data purge"
- [ ] On restart: fresh database, fresh config, onboarding flow
- [ ] Edge case: purge during active conversation (warn user, save conversation to backup)
- [ ] Edge case: purge with active model download (cancel download or complete first?)
- [ ] Edge case: partial purge (user selects specific data categories to delete)
- [ ] Edge case: system crash during purge (on restart, detect incomplete purge, offer recovery from backup)
- [ ] Edge case: insufficient disk space for backup (warn user, offer purge without backup)
- [ ] Test: purge deletes all conversation data
- [ ] Test: purge deletes all memory data
- [ ] Test: purge creates backup before deletion
- [ ] Test: restart after purge shows fresh state
- [ ] Test: partial purge deletes only selected categories

---

## 14. System & Deployment

### 14.1 Single Binary

- [ ] Linux amd64
- [ ] Linux arm64 (future)
- [ ] < 150MB uncompressed
- [ ] Embeds: Go, Wails, Svelte, SQLite, WASM, Lua, migrations, config, templates
- [ ] Headless build: `go build -tags headless`

### 14.2 Auto-Update

- [ ] GitHub Releases check
- [ ] Frequency: startup + 24h
- [ ] SemVer comparison, download, verify SHA-256, replace, restart
- [ ] Rollback on failure
- [ ] UI notification

### 14.3 Headless Mode

- [ ] `nala --headless` or `nalad`
- [ ] No UI, REST API only
- [ ] systemd service file

### 14.4 Docker

- [ ] Multi-stage alpine Dockerfile
- [ ] Docker Compose with Ollama
- [ ] Volume: /data

### 14.5 Backup/Restore

- [ ] `nala system backup /path/backup.zip`
- [ ] `nala system restore /path/backup.zip`
- [ ] Includes: DB + files + config (minus API keys)
- [ ] Confirmation required

### 14.6 Health Check

- [ ] `GET /api/v1/system/health`
- [ ] Checks: DB, vector store, providers, disk, memory
- [ ] Status: healthy, degraded, unhealthy

### 14.7 Data Directory

```
{data_dir}/
├── nala.db, vector.db, config.toml, .machine-key
├── notes/, documents/{kb_id}/
├── sandbox/data/, generated/, plugins/{id}/
├── presets.json, logs/nala.log, trash/
```

---

## 15. Quality & Polish

### 15.1 Unit Tests

- [ ] config ≥ 90% / agent ≥ 85% / model ≥ 85% / tool ≥ 80%
- [ ] memory ≥ 80% / pipeline ≥ 85% / scheduler ≥ 85% / api ≥ 75%
- [ ] crypto ≥ 95% / db ≥ 80%
- [ ] Overall ≥ 80%
- [ ] Table-driven + mocks + edge case coverage
- [ ] `go test ./... -cover` < 30s

### 15.2 Integration Tests

- [ ] DB migrations + CRUD
- [ ] Model router: registration, fallback, routing
- [ ] Agent: full loop with mock model
- [ ] Tools: reg, exec, sandbox
- [ ] Memory: vector + RAG
- [ ] Pipeline: DAG execution

### 15.3 End-to-End Tests

- [ ] Create agent → chat → receive
- [ ] Send msg → model calls tool → execute → result
- [ ] Create KB → upload → query → verify RAG
- [ ] Create pipeline → execute → verify
- [ ] Schedule task → verify completion

### 15.4 Benchmarks

- [ ] Agent loop: msgs/sec with mock
- [ ] Tool exec: tools/sec
- [ ] Vector search: queries/sec at 1K/10K/100K
- [ ] RAG: end-to-end latency
- [ ] Session load (1000 msgs): < 200ms

### 15.5 Error Handling

- [ ] Inline: form validation
- [ ] Toast: transient (3s)
- [ ] Modal: blocking
- [ ] In-chat: model/tool errors
- [ ] Error IDs (ERR-XXX) for debugging
- [ ] Retry buttons, fallback suggestions

### 15.6 Loading States

- [ ] Skeleton: messages, sessions, agents, documents
- [ ] Progress: uploads, model pulls
- [ ] Spinner: general ops
- [ ] Blinking cursor: streaming
- [ ] Timeout after 30s → error

---
## 16. Notes System

### 16.1 Storage

- [ ] Notes stored as `.md` files in `{data_dir}/notes/`
- [ ] SQLite `notes` table for metadata + FTS5
- [ ] File path: `{data_dir}/notes/{folder/}{id}.md`
- [ ] Content is the Markdown body; metadata (title, tags, etc.) is YAML frontmatter embedded in the .md file
- [ ] DB is source of truth; .md file is secondary (synced on save)
- [ ] Sync check on startup: reconcile DB ↔ filesystem (DB wins)

### 16.2 YAML Frontmatter

- [ ] `title` — string, note title
- [ ] `tags` — string[], tag list
- [ ] `created` — ISO 8601 datetime
- [ ] `modified` — ISO 8601 datetime
- [ ] `pinned` — bool
- [ ] `color` — hex color string
- [ ] `folder` — string, folder path
- [ ] `id` — UUID, matches DB primary key
- [ ] `metadata` — object, arbitrary key-value pairs
- [ ] Frontmatter delimited by `---` lines
- [ ] Missing frontmatter → extracted from filename/DB on first save

### 16.3 Folders

- [ ] Nesting: unlimited depth, `/` delimited
- [ ] Drag-reorder in UI sidebar
- [ ] Collapse/expand folders
- [ ] Empty folders visible
- [ ] Folder rename re-paths all notes inside
- [ ] Move note between folders via drag-drop or right-click menu
- [ ] Edge case: folder name with special characters (escaped)
- [ ] Edge case: rename folder with 1000+ notes (background)

### 16.4 Search

- [ ] FTS5 across all notes (title + content)
- [ ] Filter by folder prefix
- [ ] Filter by tags (AND/OR logic)
- [ ] Sort by: date created, date modified, title, relevance
- [ ] Search results: title, snippet (with highlight), folder, tags, modified date
- [ ] Search-as-you-type debounced 300ms
- [ ] Saved searches (persist in user_data)
- [ ] Edge case: zero results → "No notes found" + suggestion to create
- [ ] Edge case: search with special regex chars (escaped in FTS5)

### 16.5 Editor

- [ ] CodeMirror 6 integration
- [ ] Syntax highlighting: Markdown, code blocks, frontmatter
- [ ] Markdown preview toggle (split pane)
- [ ] Live preview: headings, bold, italic, lists, tables, links, images
- [ ] Editor toolbar: Bold, Italic, Heading, Link, Image, List, Code, Table
- [ ] Auto-close brackets, quotes
- [ ] Indentation: tabs or spaces (configurable)
- [ ] Line numbers in gutter
- [ ] Active line highlight

### 16.6 LaTeX Math

- [ ] MathJax or KaTeX for rendering
- [ ] `$$...$$` display math (block)
- [ ] `$...$` inline math
- [ ] `\[ ... \]` display math
- [ ] `\begin{equation} ... \end{equation}` numbered equation
- [ ] Common packages: amsmath, amssymb, amsfonts, mathtools
- [ ] Error rendering: show raw LaTeX with error tooltip
- [ ] Edge case: mismatched `$$` (show partial render + warning)
- [ ] Edge case: very long LaTeX expression >1000 chars (warn)

### 16.7 Image Embedding

- [ ] Drag-drop image → upload to `{data_dir}/notes/assets/`
- [ ] Paste image from clipboard → same
- [ ] File picker button in toolbar
- [ ] Supported: PNG, JPEG, WebP, GIF, SVG
- [ ] Max size: 10MB per image (configurable)
- [ ] Markdown: `![alt](assets/{uuid}.{ext})`
- [ ] Inline preview in editor and rendered view
- [ ] Image thumbnail in asset browser
- [ ] Orphaned asset cleanup on note delete
- [ ] Edge case: drag non-image file → error
- [ ] Edge case: duplicate image → skip (content-hash dedup)

### 16.8 Backlinks

- [ ] `[[wikilink]]` syntax for internal note links
- [ ] `[[wikilink|display text]]` with custom text
- [ ] Auto-discover: scan all notes for `[[...]]` references
- [ ] Backlink panel in sidebar shows notes that link to current
- [ ] Backlink count badge on note
- [ ] `[[unresolved]]` → red highlight, "Create note?" prompt
- [ ] Update on note rename (all backlinks updated)
- [ ] Test: backlinks discoverable
- [ ] Test: rename updates backlinks

### 16.9 Tags

- [ ] Tag autocomplete in editor (type `#` → dropdown)
- [ ] Tag cloud in sidebar (size = frequency)
- [ ] Tag hierarchy: `parent/child` with `/` separator
- [ ] Click tag → filter notes by tag
- [ ] Tag rename (all notes updated)
- [ ] Tag color (optional, from metadata)
- [ ] Edge case: tag with spaces (use quotes or hyphens)
- [ ] Edge case: 1000+ tags (virtualized tag cloud)

### 16.10 Versions

- [ ] Git-based auto-versioning
- [ ] `{data_dir}/notes/` is a git repo
- [ ] Auto-commit on every save
- [ ] Commit message: `note: {title}` with timestamp
- [ ] Diff viewer in UI: side-by-side or unified
- [ ] Version history timeline per note
- [ ] Restore to any previous version
- [ ] .gitignore: assets/ (too large), temporary files
- [ ] Git LFS for large assets (optional)
- [ ] Edge case: git not installed → versioning disabled, warn
- [ ] Edge case: merge conflict on concurrent edit (auto-merge or prompt)
- [ ] Test: save creates git commit
- [ ] Test: restore returns correct content

### 16.11 Export

- [ ] Single note: `.md` file download
- [ ] Batch: `.zip` of selected notes
- [ ] PDF via pandoc (requires pandoc installed)
- [ ] Export options: include/exclude frontmatter, include assets
- [ ] Export dialog: format selector, folder picker
- [ ] Export progress for batch operations
- [ ] Test: single note exports valid MD
- [ ] Test: batch ZIP contains all notes

### 16.12 Templates

- [ ] Create note from template
- [ ] Template location: `{data_dir}/notes/templates/`
- [ ] Template format: `.md` with frontmatter + body
- [ ] Template variables: `{{title}}`, `{{date}}`, `{{time}}`, `{{tags}}`
- [ ] Built-in templates: Daily Note, Meeting Notes, Project Plan, TODO List
- [ ] Custom template creation in UI
- [ ] Default template configurable in settings
- [ ] Insert template at cursor position
- [ ] Edge case: template not found → error
- [ ] Edge case: template variable not defined → leave as-is

### 16.13 Attachments

- [ ] File attachment support (any file type)
- [ ] Stored in `{data_dir}/notes/assets/{note_id}/`
- [ ] Attachment list in note properties panel
- [ ] Markdown: `[filename](assets/{note_id}/{filename})`
- [ ] Max attachment size: 50MB (configurable)
- [ ] Preview: images inline, PDF embedded, other → download link
- [ ] Attachment count badge on note
- [ ] Orphaned attachment cleanup on note delete
- [ ] Edge case: attachment with same name as existing → auto-rename

### 16.14 Trash

- [ ] Soft-delete with 30-day retention
- [ ] Deleted notes moved to `{data_dir}/notes/trash/`
- [ ] Trash folder in sidebar
- [ ] Restore from trash (moves back to original folder)
- [ ] Empty trash (permanent delete)
- [ ] Auto-purge trash older than 30 days on startup
- [ ] Edge case: trash note with same name as existing → append timestamp

### 16.15 Note Linking

- [ ] Internal links between notes via `[[id]]` or `[[slug]]`
- [ ] Link graph visualization (force-directed graph)
- [ ] Graph node: note title, edges = backlinks
- [ ] Filter graph by folder or tag
- [ ] Zoom, pan, click-to-navigate
- [ ] Graph export as PNG/SVG
- [ ] Edge case: circular links (graph handles cycles)

### 16.16 TODO Inside Notes

- [ ] Extract checkboxes `- [ ]` as tasks
- [ ] Sync with task system on save
- [ ] Task linked to note via linked_entity_id
- [ ] TODO panel: show all tasks from notes
- [ ] Check/uncheck in note updates task status
- [ ] Task completion in task system updates note checkbox
- [ ] Edge case: checkbox inside code block (not a task)
- [ ] Edge case: nested checkboxes (flat in task system)

### 16.17 Daily Notes

- [ ] Auto-create daily note on first open of the day
- [ ] Template: `Daily Notes/YYYY-MM-DD.md`
- [ ] Daily note button in sidebar
- [ ] Template variable `{{date}}` in title/body
- [ ] Journal entry section: free-form writing
- [ ] Tasks section: extracted from daily note
- [ ] "On this day" (previous year entries)
- [ ] Edge case: multiple daily notes same day (reuse existing)

### 16.18 Edge Cases

- [ ] Concurrent edits from two sessions (last-write-wins, warn on conflict)
- [ ] Symlink in notes dir (follow or skip, configurable, default skip)
- [ ] Massive single note >10MB (virtual rendering, lazy load body)
- [ ] Notes dir deleted externally (recreate, rescan)
- [ ] Filesystem full (graceful error, stop saving)
- [ ] Non-UTF-8 content (detect, convert, warn)
- [ ] Binary file with .md extension (error on open)
- [ ] Note ID collision (UUID: negligible probability)
- [ ] 10000+ notes in single folder (pagination, virtual list)

### 16.19 Tests

- [ ] Test: create note → file exists + DB record
- [ ] Test: edit note → file content updated
- [ ] Test: delete note → moved to trash
- [ ] Test: restore from trash → back in original folder
- [ ] Test: FTS5 search finds notes
- [ ] Test: backlinks discovered correctly
- [ ] Test: template variable substitution
- [ ] Test: attachment upload and reference
- [ ] Test: TODO extraction creates tasks
- [ ] Test: rename note updates backlinks
- [ ] Test: 10MB note loads without crash
- [ ] Test: concurrent edit produces no data loss

---

## 17. Calendar & Reminders

### 17.1 Storage

- [ ] Local iCal file: `{data_dir}/nala.ics`
- [ ] SQLite `calendars` table with all event metadata
- [ ] RRULE expansion done at query time (not pre-expanded)
- [ ] Recurrence exceptions stored in `recurrence_exceptions` JSON array
- [ ] All times stored in UTC, displayed in local timezone
- [ ] Timezone database: Go's `time/tzdata` embedded

### 17.2 Calendar Views

- [ ] Month view: full month grid, events as colored bars
- [ ] Week view: 7-day grid with hourly slots
- [ ] Day view: single day with hourly slots
- [ ] Agenda view: scrollable list of upcoming events
- [ ] Mini-calendar in sidebar (month thumbnail)
- [ ] View navigation: prev/next, go to today
- [ ] Today indicator in all views
- [ ] Drag events to change time (in week/day view)
- [ ] Drag edges to resize event
- [ ] Edge case: event spanning multiple days (displayed across days)

### 17.3 Event CRUD

- [ ] Create: title required, start_time required, end_time optional
- [ ] Edit: all fields editable, recurrence exceptions tracked
- [ ] Delete: confirm dialog, option to delete all/this/future for recurring
- [ ] Quick-create: click empty slot → fill title + auto-time
- [ ] Event details panel: full info, edit button, delete button
- [ ] Recurrence editor: daily/weekly/monthly/yearly presets + custom RRULE
- [ ] Color picker per event
- [ ] Location field with optional maps link
- [ ] Description with Markdown support
- [ ] Test: create → edit → delete cycle

### 17.4 Recurrence

- [ ] Supported RRULE freq: DAILY, WEEKLY, MONTHLY, YEARLY
- [ ] INTERVAL, COUNT, UNTIL, BYDAY, BYMONTHDAY, BYMONTH, BYSETPOS
- [ ] EXDATE (exception dates) from recurrence_exceptions
- [ ] Recurrence expansion: generate N occurrences (max 730, ~2 years)
- [ ] Virtual occurrences: not stored in DB, generated at query time
- [ ] Single instance edit: create EXDATE + new non-recurring event
- [ ] Edit all future: split into two recurring events at change point
- [ ] Delete single instance: add EXDATE
- [ ] Delete all future: set UNTIL to day before deletion
- [ ] Edge case: no end (COUNT=0/UNTIL unset) — expand 2 years
- [ ] Edge case: monthly on day 31 for short months (roll to last day)
- [ ] Edge case: yearly on Feb 29 for non-leap years (roll to Feb 28)

### 17.5 Reminders

- [ ] Popup notification at event time (libnotify)
- [ ] Sound: configurable WAV/MP3
- [ ] Email summary of today's events at configurable time
- [ ] Default reminder: 10 minutes before (configurable per event)
- [ ] Multiple reminders per event (e.g., 1 day + 1 hour)
- [ ] Reminder dismissed → next occurrence for recurring events
- [ ] Test: reminder fires at correct time

### 17.6 Reminder Snooze

- [ ] Snooze options: 5 min, 15 min, 1 hour, 1 day, custom
- [ ] Snooze updates remind_at, sets status=snoozed
- [ ] Snooze within app notification popup
- [ ] Max snooze: 7 days (prevent infinite)
- [ ] Test: snooze reschedules correctly

### 17.7 Missed Reminder Detection

- [ ] On startup: query reminders WHERE remind_at < now AND status=pending
- [ ] Show missed reminder dialog with "Dismiss" or "Reschedule"
- [ ] Fire missed reminders automatically (configurable)
- [ ] Edge case: startup after 30 days offline → fire only first 10 missed

### 17.8 Calendar Integration

- [ ] CalDAV sync (read/write)
- [ ] Google Calendar integration (read-only via Google API)
- [ ] Sync interval: configurable (default 15 min)
- [ ] Conflict resolution: last-write-wins with timestamp comparison
- [ ] Credential storage: OAuth tokens encrypted via AES-256-GCM
- [ ] Sync status indicator per calendar
- [ ] Force sync button in UI
- [ ] Edge case: CalDAV server down (retry with backoff, 3 retries)
- [ ] Edge case: sync conflict (user prompt on desktop)

### 17.9 Multiple Calendars

- [ ] Local calendar (default) + zero or more remote calendars
- [ ] Calendar list in sidebar with show/hide toggle
- [ ] Overlay view: multiple calendars shown simultaneously
- [ ] Color-coded per calendar
- [ ] Events show origin calendar name in tooltip
- [ ] Calendar settings: name, color, sync URL, enabled
- [ ] Disabled calendar: events hidden, not synced

### 17.10 Natural Language Parsing

- [ ] "meeting tomorrow at 3pm" → event tomorrow 15:00, 1h duration
- [ ] "lunch every Friday at noon" → recurring event
- [ ] "dentist next Tuesday at 10am for 2 hours" → specific event
- [ ] "remind me to call John in 30 minutes" → reminder
- [ ] Backend: rule-based parser with timex extraction
- [ ] Fallback: model-assisted parsing with structured output
- [ ] Display parsed result for user confirmation before creation
- [ ] Test: natural language creates correct event

### 17.11 Agent Calendar Tools

- [ ] `calendar.list` — list events for date range, supports natural language date
- [ ] `calendar.create` — create event, supports natural language input
- [ ] `calendar.update` — update event by ID
- [ ] `calendar.delete` — delete event by ID
- [ ] `calendar.search` — search events by title/description
- [ ] All tools accept natural language date strings
- [ ] Test: agent creates event via tool

### 17.12 Overlap Detection

- [ ] On create/update: check existing events in same time range
- [ ] Overlap warning: "This event overlaps with: {event titles}"
- [ ] Overlap severity: mild (<30min overlap), significant (>30min)
- [ ] User choice: create anyway, adjust time, cancel
- [ ] No overlap check for all_day events

### 17.13 Timezone Handling

- [ ] All event times stored as UTC ISO 8601 in DB
- [ ] Timezone column records original TZ
- [ ] Display converted to user's local TZ (from settings or system)
- [ ] Timezone picker on event create/edit
- [ ] Floating time (no TZ) → treated as local time
- [ ] Edge case: DST transition (events at 2:30am on DST change → valid)
- [ ] Edge case: event created in TZ A, viewed in TZ B (correct conversion)
- [ ] Edge case: IANA timezone not found (fallback to UTC, warn)

### 17.14 Edge Cases

- [ ] DST transitions: no event lost or duplicated
- [ ] Leap year: Feb 29 handled correctly
- [ ] Event at midnight: displayed as 12:00am, not spanning days
- [ ] Event with end before start (validation error)
- [ ] Orphaned reminder after event deletion (cascade delete)
- [ ] Recurring event with 1000+ occurrences (virtual expansion, paginate)
- [ ] Calendar file corrupted (rebuild from DB)
- [ ] Timezone DB update (embedded in binary, ship with releases)

### 17.15 Tests

- [ ] Test: create event → retrieved correctly
- [ ] Test: recurring event expands to N occurrences
- [ ] Test: single instance edit creates exception
- [ ] Test: delete all/this/future works
- [ ] Test: overlap detection triggers warning
- [ ] Test: timezone conversion correct
- [ ] Test: natural language creates event
- [ ] Test: reminder fires and shows notification
- [ ] Test: snooze reschedules reminder
- [ ] Test: missed reminder detected on startup
- [ ] Test: CalDAV sync round-trip (mock server)

---

## 18. Email Integration

### 18.1 Account Management

- [ ] Add account: SMTP + IMAP settings
- [ ] Edit account: change settings, re-auth
- [ ] Remove account: remove all emails + credentials
- [ ] Account status: connected, disconnected, error
- [ ] Test connection button
- [ ] Multiple accounts supported
- [ ] Per-account folder sync settings
- [ ] Account color label in UI

### 18.2 Credential Encryption

- [ ] SMTP/IMAP passwords encrypted at rest via AES-256-GCM
- [ ] OAuth tokens for Gmail/Outlook encrypted same way
- [ ] Plaintext passwords never logged or stored
- [ ] Encryption key derived from machine seed
- [ ] Credential hint: show domain + username only
- [ ] Test: encrypt/decrypt round-trip for IMAP password

### 18.3 IMAP Sync

- [ ] Folder list: INBOX, Sent, Drafts, Archive, Spam, Trash + custom
- [ ] Message list: paginated, batch fetch 50 at a time
- [ ] Body fetch: on demand (not bulk)
- [ ] Attachment download: on demand
- [ ] Sync strategy: UID-based incremental sync
- [ ] Sync triggers: startup, periodic (60s), manual
- [ ] New email count badge on account
- [ ] Real-time push: IDLE command (IMAP)
- [ ] Edge case: connection drop → exponential backoff, max 10 retries
- [ ] Edge case: large mailbox >10K messages (pagination, batch sync)
- [ ] Edge case: SSL certificate error (option to accept, warn)
- [ ] Edge case: IMAP server rate limit (back off, retry)

### 18.4 SMTP Send

- [ ] Compose: To, CC, BCC, Subject, Body
- [ ] Reply: pre-fill To, Subject = "Re: {original}"
- [ ] Reply all: To + CC from original
- [ ] Forward: Subject = "Fwd: {original}", attach original
- [ ] Attachment support: up to 25MB total
- [ ] Send progress: "Sending..." → "Sent" / "Failed"
- [ ] Save as draft
- [ ] Cancel sending (if queued)
- [ ] Edge case: SMTP server down (queue, retry, notify)
- [ ] Edge case: attachment too large (warn before send)
- [ ] Edge case: invalid recipient address (validation on send)

### 18.5 Email Threading

- [ ] Threading by Message-ID / In-Reply-To / References headers
- [ ] Thread view: hierarchical, expandable
- [ ] Thread count badge in message list
- [ ] Collapse/expand entire thread
- [ ] Unread count per thread
- [ ] Flat mode (no threading) toggle
- [ ] Edge case: missing Message-ID (generate from content hash)
- [ ] Edge case: circular References (cap at 50 depth)

### 18.6 Search

- [ ] FTS5 across from, to, subject, body_text
- [ ] Filter by: folder, account, is_read, is_starred, date range
- [ ] Search-as-you-type debounced 300ms
- [ ] Saved searches
- [ ] Combined search with notes (optional)
- [ ] Test: FTS5 finds email by subject
- [ ] Test: filter by folder returns correct subset

### 18.7 Filters & Rules

- [ ] Rule conditions: from, to, subject (contains, matches, equals)
- [ ] Rule actions: mark read, archive, delete, move to folder, label, forward
- [ ] Rule evaluation: ordered, first match wins
- [ ] Run rules on new email (auto)
- [ ] Run rules manually on folder
- [ ] Rule import/export (JSON)
- [ ] Test: filter rule auto-archives matching email
- [ ] Test: multiple rules execute in order

### 18.8 Templates

- [ ] Save email as template (subject + body)
- [ ] Template variables: `{{name}}`, `{{email}}`, `{{date}}`, `{{subject}}`
- [ ] Insert template in compose window
- [ ] Manage templates: list, edit, delete
- [ ] Built-in templates: "Out of office", "Thanks", "Meeting confirmation"

### 18.9 Attachments

- [ ] Preview inline: images, PDF (via embedded viewer)
- [ ] Download attachment to local filesystem
- [ ] Save attachment to notes (save to `{data_dir}/notes/assets/`)
- [ ] Attachment list in email detail
- [ ] Attachment size in list
- [ ] Warning for >10MB attachments
- [ ] Blocked extensions: .exe, .bat, .cmd, .scr, .vbs (configurable)

### 18.10 PGP Encryption

- [ ] PGP support via gpg binary
- [ ] Key management: import, list, sign, trust
- [ ] Encryption on send if recipient key available
- [ ] Decryption on receive if private key available
- [ ] Key search: keyserver lookup
- [ ] Status indicators: encrypted (lock icon), signed (check icon)
- [ ] Edge case: gpg not installed → PGP features hidden
- [ ] Edge case: unknown sender key → warning, display encrypted blob

### 18.11 Agent Integration

- [ ] `email.send` — send email with agent-composed content
- [ ] `email.inbox` — list inbox, filterable
- [ ] `email.read` — read full email by ID
- [ ] `email.search` — search across all folders
- [ ] `email.summarize` — summarize thread or inbox
- [ ] `email.draft` — draft reply for user review
- [ ] Agent can: "summarize my inbox", "draft reply to Alice", "find emails about project X"
- [ ] Test: agent sends email via tool

### 18.12 Natural Language

- [ ] "find emails from Alice about the project" → search + filter
- [ ] "draft a reply to Bob's last email" → reply draft
- [ ] "show me unread emails from today" → filter
- [ ] "send an email to team: meeting at 3pm" → compose + send
- [ ] Backend: intent extraction from query, map to email operations
- [ ] User confirmation before sending

### 18.13 Edge Cases

- [ ] Large mailbox >10K messages (lazy load, pagination)
- [ ] Connection drops during sync (resume from last UID)
- [ ] SSL/TLS errors (log, notify user, offer to disable cert check)
- [ ] IMAP server rate limits (backoff, queue)
- [ ] SMTP rate limits (queue with delay)
- [ ] Duplicate emails (dedup by Message-ID)
- [ ] Email with no body (empty body_text, try body_html)
- [ ] HTML-only email (extract text via HTML→markdown conversion)
- [ ] Very long subject >998 chars (truncate, per RFC 2822)
- [ ] Attachment with malicious filename (sanitize, block path traversal)

### 18.14 Tests

- [ ] Test: account CRUD
- [ ] Test: IMAP sync with mock server
- [ ] Test: SMTP send with mock server
- [ ] Test: reply pre-fills fields
- [ ] Test: thread grouping by Message-ID
- [ ] Test: FTS5 search across fields
- [ ] Test: filter rule auto-archives email
- [ ] Test: agent sends email via tool
- [ ] Test: credential encrypt/decrypt round-trip

---

## 19. Skills System

### 19.1 Architecture

- [ ] Skills are agent "plugins" that provide specialized capabilities
- [ ] Distinct from generic tools: skills have lifecycles, manifests, and isolation
- [ ] Each skill can expose: tools, system prompts, config schema, knowledge bases
- [ ] Skills run in sandboxed environments
- [ ] Runtime languages: Lua, WASM, Python

### 19.2 Skill Format

- [ ] Entrypoint file: `main.lua`, `main.wasm`, or `main.py`
- [ ] Manifest: `manifest.yaml` in skill root directory
- [ ] Manifest fields:
  - [ ] `id` — unique identifier, `^[a-z][a-z0-9_-]{2,48}$`
  - [ ] `name` — human-readable name
  - [ ] `version` — semver string
  - [ ] `description` — short description
  - [ ] `author` — author name
  - [ ] `license` — SPDX license identifier
  - [ ] `category` — coding, writing, research, productivity, entertainment, utility
  - [ ] `entrypoint` — relative path to main file
  - [ ] `language` — lua, python, wasm
  - [ ] `dependencies` — map of skill_id → version constraint
  - [ ] `permissions` — map of permission → boolean
  - [ ] `config_schema` — JSON Schema for skill configuration
  - [ ] `tools` — array of tool definitions (id, name, description, parameters)
  - [ ] `system_prompt` — additional system prompt content (optional)
- [ ] Manifest validation on install
- [ ] Test: valid manifest parses correctly

### 19.3 Categories

- [ ] `coding` — code review, generation, refactoring, debugging
- [ ] `writing` — prose, poetry, editing, style checking
- [ ] `research` — web research, paper summarization, fact checking
- [ ] `productivity` — task management, email, calendar integration
- [ ] `entertainment` — games, jokes, stories, creative writing
- [ ] `utility` — math, data analysis, formatting, conversion

### 19.4 Lifecycle

- [ ] Install: download/unpack → validate manifest → check dependencies → register in DB
- [ ] Enable: verify entrypoint exists → instantiate runtime → register tools → set is_enabled=true
- [ ] Disable: unregister tools → close runtime → set is_enabled=false
- [ ] Uninstall: disable → remove files → remove DB record
- [ ] Update: download new version → backup old → install → enable → remove backup on success
- [ ] Status: installed, enabled, disabled, error, updating
- [ ] UI: skill card with status badge, enable/disable toggle, update button, uninstall button
- [ ] Test: install → enable → disable → uninstall cycle

### 19.5 Skill Store

- [ ] Bundled skills: shipped with Nala binary
- [ ] Community repo URL: configurable, default `https://skills.nala.ai/`
- [ ] Repository format: JSON index of {id, name, version, description, category, downloads}
- [ ] Browse: list, search, filter by category
- [ ] Skill detail: description, version, author, download count, rating
- [ ] Install from store: download + verify checksum + install
- [ ] Test: install from mock store

### 19.6 Skill Isolation

- [ ] Lua: restricted environment via gopher-lua, no os/io/debug modules
- [ ] WASM: wazero runtime, 64MB memory limit, host function gating
- [ ] Python: subprocess with JSON-RPC, RLIMIT_AS 256MB, RLIMIT_CPU 30s
- [ ] All languages: no filesystem access outside `{data_dir}/skills/{id}/`
- [ ] Network access: controlled by permissions manifest (off by default)
- [ ] Crash isolation: skill crash kills only that skill's runtime
- [ ] Hang detection: timeout per tool call (default 30s)
- [ ] Test: Lua cannot access os.execute
- [ ] Test: WASM memory limit enforced

### 19.7 Skill Permissions

- [ ] Declared in manifest: `permissions: { network: false, filesystem: true, ... }`
- [ ] Permission categories: network, filesystem, process, audio, camera, location
- [ ] User must approve permissions on install
- [ ] Permission levels: allow, ask, deny
- [ ] Revocable: change permissions after install
- [ ] Excessive permissions warning: "This skill requests {N} permissions"

### 19.8 Skill Auto-Update

- [ ] Check on startup: query store for newer versions
- [ ] Periodic check: every 24 hours (configurable)
- [ ] Update process: download → verify SHA-256 → backup old → install → test
- [ ] Rollback: if new version fails to enable, restore backup
- [ ] Auto-update toggle per skill
- [ ] Auto-update global toggle in settings
- [ ] Test: auto-update checks and installs

### 19.9 Built-in Skills

- [ ] `code_reviewer` — reviews code changes, suggests improvements
- [ ] `research_assistant` — web research with citation
- [ ] `writing_coach` — style guide, grammar checking, tone analysis
- [ ] `data_analyst` — CSV/JSON analysis, chart generation
- [ ] `translator` — multi-language translation
- [ ] `summarizer` — document/URL summarization
- [ ] Built-in skills cannot be uninstalled (only disabled)

### 19.10 Skill Marketplace UI

- [ ] Browse tab: grid of skills, search, filter by category
- [ ] Skill detail: description, version history, screenshots, reviews
- [ ] Install button → permission prompt → progress bar
- [ ] Installed tab: list of installed skills, update badges
- [ ] Ratings and reviews (optional, requires community account)

### 19.11 Custom Skill Development

- [ ] Editor: create skill manifest + code in UI
- [ ] Template: "New Skill" generates boilerplate for chosen language
- [ ] Local testing: run skill in sandbox, see tool call results
- [ ] Export: bundle as .zip for sharing
- [ ] Import: install from .zip file
- [ ] Debug mode: verbose logging, runtime introspection
- [ ] Documentation: skill development guide in-app

### 19.12 Skill Dependencies

- [ ] `dependencies` field in manifest: { skill_id: version_constraint }
- [ ] Version constraints: ">=1.0.0", "~1.2.0", "^2.0.0"
- [ ] Dependency resolution: install deps first, verify versions
- [ ] Circular dependency detection (reject install)
- [ ] Dependency update triggers for dependents
- [ ] Test: dependency resolution installs in correct order

### 19.13 Edge Cases

- [ ] Skill conflict: two skills expose same tool ID (first-enable wins, warn)
- [ ] Skill crash: runtime restarted, does not affect other skills
- [ ] Skill with excessive permissions: user warning, highlight dangerous perms
- [ ] Missing entrypoint file: error on enable, stay disabled
- [ ] Malformed manifest: reject install with specific error
- [ ] Skill update breaks API compatibility: rollback + notify
- [ ] Zero-byte skill: reject on install
- [ ] Skill with infinite loop: timeout kills, log warn
- [ ] Skill store unavailable: offline install from .zip still works

### 19.14 Tests

- [ ] Test: install skill from .zip
- [ ] Test: enable/disable skill registers/unregisters tools
- [ ] Test: skill tool call returns correct result
- [ ] Test: Lua skill cannot escape sandbox
- [ ] Test: WASM skill respects memory limit
- [ ] Test: permission enforcement blocks disallowed operations
- [ ] Test: auto-update downloads and installs new version
- [ ] Test: dependency resolution installs in order
- [ ] Test: circular dependency rejected

---

## 20. Editor (Markdown + LaTeX)

### 20.1 Architecture

- [ ] Dedicated editor panel (separate from chat input)
- [ ] CodeMirror 6 integration with Svelte wrapper
- [ ] ProseMirror alternative evaluated (CodeMirror preferred for simplicity)
- [ ] Split pane: source (left) + preview (right), resizable
- [ ] Toggle: source-only, preview-only, side-by-side
- [ ] Full-screen mode (F11)

### 20.2 Markdown Features

- [ ] Headings: # to ###### with outline panel
- [ ] Bold: **text** or Ctrl+B
- [ ] Italic: *text* or Ctrl+I
- [ ] Strikethrough: ~~text~~
- [ ] Tables: pipe syntax, auto-format on tab
- [ ] Lists: ordered, unordered, nested, task lists
- [ ] Footnotes: [^1] syntax with tooltip on hover
- [ ] Blockquotes: > syntax with nesting
- [ ] Horizontal rules: ---, ***, ___
- [ ] Links: [text](url) or Ctrl+K with dialog
- [ ] Images: ![alt](url) with inline preview
- [ ] Keyboard: <kbd>Ctrl+S</kbd>
- [ ] Markdown formatting toolbar

### 20.3 LaTeX Math

- [ ] `$...$` inline math
- [ ] `$$...$$` display math
- [ ] `\[ ... \]` display math
- [ ] `\begin{equation} ... \end{equation}` numbered
- [ ] KaTeX for rendering (not MathJax — faster)
- [ ] Auto-render on preview pane update
- [ ] Error in formula: red highlight + error message
- [ ] Common packages: amsmath, amssymb, amsfonts, mathtools, physics
- [ ] Test: inline math renders correctly
- [ ] Test: display math renders with equation number

### 20.4 Code Blocks

- [ ] Triple backtick with language identifier
- [ ] Syntax highlighting for 50+ languages via CodeMirror
- [ ] Copy button (copies code content only)
- [ ] Line numbers in code blocks
- [ ] Wrap toggle for long lines
- [ ] Code block language selector in toolbar
- [ ] Embed code block output (run button for supported languages, optional)
- [ ] Test: syntax highlighting renders for Python

### 20.5 Mermaid Diagrams

- [ ] ````mermaid → rendered SVG in preview
- [ ] Supported diagram types: flowchart, sequence, class, state, gantt, pie, ERD, journey, gitgraph
- [ ] Mermaid version: bundled (no network required)
- [ ] Error: invalid diagram syntax → show raw + error message
- [ ] Export diagram as PNG/SVG
- [ ] Zoom/pan on large diagrams
- [ ] Test: flowchart renders as SVG

### 20.6 Auto-Complete

- [ ] `[[` → note link autocomplete (search notes by title)
- [ ] `#` → tag autocomplete (existing tags)
- [ ] `:emoji:` → emoji picker autocomplete
- [ ] `@` → mention autocomplete (agents, sessions) (future)
- [ ] Tab to accept, Esc to dismiss
- [ ] Test: [[ triggers note search

### 20.7 Keybindings

- [ ] `Ctrl+B` — bold
- [ ] `Ctrl+I` — italic
- [ ] `Ctrl+K` — insert link
- [ ] `Ctrl+Shift+P` — toggle preview
- [ ] `Ctrl+S` — save file
- [ ] `Ctrl+N` — new file
- [ ] `Ctrl+W` — close tab
- [ ] `Ctrl+F` — search in file
- [ ] `Ctrl+H` — search and replace
- [ ] `Ctrl+Z` / `Ctrl+Shift+Z` — undo / redo
- [ ] `Ctrl+D` — delete line
- [ ] `Ctrl+/` — toggle comment
- [ ] `Tab` / `Shift+Tab` — indent / outdent
- [ ] `Alt+Up/Down` — move line up/down
- [ ] `Ctrl+Shift+M` — toggle mermaid preview
- [ ] Customizable keybindings in settings
- [ ] Vim mode (CodeMirror vim plugin) — optional

### 20.8 File Operations

- [ ] Open file: file dialog or drag-drop into editor
- [ ] Save: save to current path (Ctrl+S)
- [ ] Save As: dialog to choose location
- [ ] Rename: right-click tab → rename
- [ ] Delete: right-click tab → delete (confirm)
- [ ] Recent files list
- [ ] Auto-save: every 30 seconds (configurable)
- [ ] Auto-save on focus loss
- [ ] File type detection by extension
- [ ] Encoding detection: UTF-8, UTF-16, Latin-1 (with fallback)

### 20.9 Multi-Cursor Editing

- [ ] Alt+Click → add cursor
- [ ] Ctrl+Alt+Down/Up → add cursor below/above
- [ ] Ctrl+D → select next occurrence
- [ ] Ctrl+Shift+L → select all occurrences
- [ ] Multi-cursor: editing, selection, pasting

### 20.10 Spell Check

- [ ] Hunspell integration (system dictionary)
- [ ] Underline misspelled words (red wavy)
- [ ] Right-click → suggestions
- [ ] Add to dictionary
- [ ] Language detection per file
- [ ] Dictionary path: system default, override in settings
- [ ] Edge case: hunspell not installed → spell check disabled, warn
- [ ] Edge case: mixed language document (primary dictionary only)

### 20.11 Focus Mode

- [ ] Hide everything except editor pane
- [ ] Dimmed background, centered content
- [ ] Typewriter mode: current line always centered
- [ ] Toggle via Ctrl+Shift+F or menu
- [ ] Exit via Esc or toggle again

### 20.12 Statistics

- [ ] Character count (with/without spaces)
- [ ] Word count
- [ ] Line count
- [ ] Reading time (at 250 wpm)
- [ ] Paragraph count
- [ ] Statistics panel or status bar display
- [ ] Real-time update

### 20.13 Export

- [ ] PDF via pandoc (requires pandoc)
- [ ] HTML with CSS styling
- [ ] Plain text (strips formatting)
- [ ] DOCX via pandoc
- [ ] Export range: selection, current file, multiple files
- [ ] Export options: table of contents, syntax highlighting, custom CSS
- [ ] Test: export to HTML produces valid document
- [ ] Test: export to PDF with pandoc

### 20.14 Tabs

- [ ] Multiple files open in tabs
- [ ] Tab: filename (truncated), modified indicator (dot), close button
- [ ] Drag to reorder tabs
- [ ] Middle-click to close
- [ ] Tab overflow: scrollable tabs
- [ ] Tab context menu: close, close others, close right, close all
- [ ] Session persistence: reopen tabs on restart
- [ ] Max tabs: 20 (configurable)

### 20.15 File Tree Sidebar

- [ ] Folder navigation tree
- [ ] File icons by type
- [ ] Right-click context menu: new file, new folder, rename, delete
- [ ] Drag-drop files to move
- [ ] Filter by filename
- [ ] Show hidden files toggle
- [ ] Auto-refresh on filesystem changes
- [ ] Collapse/expand all

### 20.16 Git Integration

- [ ] Status indicators: modified (M), added (A), deleted (D), untracked (U)
- [ ] Diff gutter: green (added lines), red (deleted lines), blue (modified)
- [ ] Inline diff on save (compare with previous commit)
- [ ] Git blame annotations in gutter (shows author, date)
- [ ] Requires git in open directory (detect .git at or above file)
- [ ] Edge case: no git repo → git features hidden

### 20.17 Edge Cases

- [ ] Very large files >10MB (virtual rendering, only render visible lines)
- [ ] Binary files opened in editor (show hex view or error)
- [ ] Encoding detection failure (default to UTF-8, show warning)
- [ ] File modified externally (detect change, prompt reload)
- [ ] Permission denied on save (show error, suggest Save As)
- [ ] Disk full on save (error, keep in-memory state)
- [ ] Character-level undo across 10K+ edits (memory managed)
- [ ] RTL text (basic support, no full RTL layout)
- [ ] Zero-width characters in file (visible highlight)

### 20.18 Tests

- [ ] Test: open file renders content
- [ ] Test: save file writes to disk
- [ ] Test: Ctrl+B toggles bold
- [ ] Test: [[ triggers note autocomplete
- [ ] Test: $$ math renders in preview
- [ ] Test: mermaid block renders SVG
- [ ] Test: multi-cursor editing works
- [ ] Test: spell check underlines misspelled words
- [ ] Test: word count accurate
- [ ] Test: 10MB file opens without lag
- [ ] Test: export to HTML produces valid output

---

## 21. Personalization Engine

### 21.1 User Profile Model

- [ ] Profile stored in `user_data` table with category=preference/fact/behavior/stat
- [ ] Profile built from: conversations, tool usage, feedback, explicit settings
- [ ] Profile dimensions:
  - [ ] `communication_style` — formal, casual, technical
  - [ ] `verbosity` — concise, balanced, detailed
  - [ ] `expertise_level` — beginner, intermediate, expert
  - [ ] `interests` — map of tag → weight (0.0-1.0)
  - [ ] `communication_channel` — text, image, code, any
  - [ ] `preferred_model` — model ID string
  - [ ] `preferred_temperature` — float 0.0-2.0
  - [ ] `response_format` — text, markdown, code-only
  - [ ] `greeting_preference` — formal, casual, none
- [ ] Default profile if no data collected

### 21.2 Learning Sources

- [ ] Explicit: settings, "Remember this" button, direct preference setting
- [ ] Implicit: conversation analysis (style, length, topics)
- [ ] Implicit: tool choice patterns (which tools, how often)
- [ ] Implicit: response feedback (thumbs up/down, regen rate)
- [ ] Implicit: engagement metrics (read time, follow-up questions)
- [ ] Feedback collection: inline thumbs up/down on every response
- [ ] Feedback collection: rating dialog on session end
- [ ] Test: explicit preference saved correctly
- [ ] Test: implicit inference from conversation

### 21.3 Inference Frequency

- [ ] Every 10 messages: run lightweight inference
- [ ] On session end: full profile update
- [ ] On demand: "Update my profile" button
- [ ] Inference engine: rule-based + model-assisted (configurable)
- [ ] Rule-based: keyword detection, style analysis, tool frequency
- [ ] Model-assisted: LLM analyzes conversation for profile cues
- [ ] Profile version: increment on each update
- [ ] Profile change log: track what changed and when

### 21.4 Model Persona Adjustment

- [ ] System prompt modified based on user profile
- [ ] Communication style: prepend "Use a {style} tone"
- [ ] Verbosity: prepend "Be {concise/detailed}"
- [ ] Expertise level: adjust technical terms and explanations
- [ ] Interests: emphasize matching domains
- [ ] Persona injection: after system prompt, before conversation
- [ ] Adjustable per dimension (user can lock dimensions)
- [ ] Test: persona adjustment changes response style

### 21.5 UI Adaptation

- [ ] Layout defaults: sidebar width, chat font size, theme
- [ ] Theme preference: light/dark/system (from profile or explicit)
- [ ] Font size: default from profile
- [ ] Model suggestion: "You usually prefer {model}"
- [ ] Tool suggestions: "You often use {tool}"
- [ ] Default agent: based on usage frequency
- [ ] Adaptive UI respects user overrides

### 21.6 Content Recommendations

- [ ] Which knowledge bases to search (preferred KBs)
- [ ] Which tools to suggest (frequently used tools)
- [ ] Which agents to recommend (based on task type)
- [ ] Recommendation engine: frequency + recency scoring
- [ ] Hide low-scoring recommendations
- [ ] Reset recommendations on profile change

### 21.7 Privacy Controls

- [ ] Collection opt-out: disable per dimension or per source
- [ ] Data stored: profile dimensions + raw learning data
- [ ] Data retention: raw data 90 days, profile indefinitely
- [ ] Purge history: delete raw learning data, keep profile
- [ ] Export profile: JSON of all dimensions
- [ ] Import profile: restore from export
- [ ] Privacy level in settings: none (collect all) / low / medium / high (profile only) / maximum (off)
- [ ] UI: "What Nala knows about you" page
- [ ] UI: per-dimension confidence display
- [ ] UI: per-dimension edit/clear/override

### 21.8 Profile Viewer

- [ ] Settings page: "Personalization" tab
- [ ] Show each dimension with current value and confidence
- [ ] Edit dimension: override value, lock dimension
- [ ] Delete dimension: reset to default
- [ ] Source breakdown: how each value was learned
- [ ] Timeline: when dimensions changed
- [ ] Raw data viewer: see collected learning data (opt-in)

### 21.9 Opt-Out

- [ ] Per-dimension: disable learning for specific dimensions
- [ ] Per-source: disable learning from specific sources (e.g., tool usage only)
- [ ] Per-session: incognito mode (no learning)
- [ ] Global: disable personalization entirely
- [ ] Opt-out respected immediately

### 21.10 Edge Cases

- [ ] Conflicting signals: same user uses both formal and casual language (weighted average, low confidence)
- [ ] Low-confidence inferences: threshold <0.3 → not applied
- [ ] User changes mind: manual override resets learning for that dimension
- [ ] Profile reset: clear all dimensions → default
- [ ] Multiple users on same machine: single profile, or user switching (future)
- [ ] Inference from very short conversation (min 5 messages required)
- [ ] Model-assisted inference fails: fall back to rule-based
- [ ] Profile data corruption: recover from backup

### 21.11 Tests

- [ ] Test: explicit preference saved and retrieved
- [ ] Test: implicit inference from conversation
- [ ] Test: persona adjustment modifies system prompt
- [ ] Test: profile export produces valid JSON
- [ ] Test: privacy opt-out prevents collection
- [ ] Test: conflicting signals produce low confidence
- [ ] Test: profile reset returns to defaults

### 21.12 Episodic Memory — "Nala Remembers"

- [ ] This is the core of Nala's personal memory: remembering specific events, conversations, and facts about the user's life
- [ ] Every user message and important model response is analyzed for episodic content
- [ ] Episode types: meeting, conversation, event, task, idea, preference, fact, relationship, project
- [ ] Episode stored as: {type, timestamp, entities[], summary, raw_snippet, source, importance}
- [ ] Importance scoring: explicit ("remember this" = 1.0), implicit (user emphasis, repetition, emotional charge = 0.3-0.9), default = 0.5
- [ ] High importance ( > 0.8 ) episodes stored permanently
- [ ] Low importance episodes: stored for 30 days, then summarized or pruned
- [ ] Episode linking: auto-link episodes that share entities → build narrative chains
- [ ] Narrative chain: "started project X" → "worked on X" → "finished X" → "demoed X at conference"
- [ ] Episode retrieval: triggered by relevant user query
- [ ] Retrieval combines: vector similarity + entity match + temporal relevance + narrative chain position
- [ ] "Remember when..." trigger: user starts query with "Remember when" → fetch narrative chain
- [ ] Episodic memory consolidation: weekly, group related episodes into narrative summaries
- [ ] Episodic memory UI: timeline view of remembered events, filterable by type/tag
- [ ] Forget: user can delete specific episodes or entire narrative chains
- [ ] Test: "Remember when I started the encryption project?" returns episode chain
- [ ] Test: implicit importance scoring captures user emphasis

### 21.13 "Who/What/When/Where" Query Resolution

- [ ] Specialized handler for personal knowledge queries
- [ ] Query patterns detected:
  - [ ] "Who is/was {X}?" → find person entity, return context
  - [ ] "What was that thing about {topic}?" → find project/event/concept, return details
  - [ ] "When did {event} happen?" → find event, return date+context
  - [ ] "Where did {event} happen?" → find event with location, return location+context
  - [ ] "What did {person} say about {topic}?" → find person+conversation about topic
  - [ ] "Tell me about {person/project}" → comprehensive entity summary
- [ ] Resolution pipeline:
  - [ ] Step 1: Parse query into {question_type, target_entity, constraints, time_range}
  - [ ] Step 2: Entity search across all sources (conversations, emails, calendar, notes)
  - [ ] Step 3: If entity ambiguous (multiple matches), rank by recency+relevance+confidence
  - [ ] Step 4: If entity not found, expand search with fuzzy matching, synonyms
  - [ ] Step 5: Retrieve context for best matching entity
  - [ ] Step 6: Format response: "I think you're referring to {name}. Here's what I know..."
- [ ] Ambiguity handling:
  - [ ] "Do you mean the {project} from last month or the one from last year?"
  - [ ] "There are two people named Sarah in your contacts — the developer or the designer?"
- [ ] Unknown gracefully: "I don't have any information about {query}. Would you like to tell me?"
- [ ] Test: "Who is Sarah?" returns person entity with context
- [ ] Test: "What was the encryption project?" returns project with timeline
- [ ] Test: "When did I start the encryption project?" returns date from conversation
- [ ] Test: ambiguous entity prompts clarification

### 21.14 Personal Knowledge Graph

- [ ] Entity-relationship graph of user's personal context
- [ ] Nodes: person, project, organization, event, technology, concept, location
- [ ] Edges: works_on, met_at, talked_about, attended, created, uses, related_to
- [ ] Edge properties: strength (0.0-1.0), first_seen, last_seen, source_count, context_snippets[]
- [ ] Graph built automatically from entity extraction (§8.11)
- [ ] Graph query: "How are {entity_A} and {entity_B} related?" → find path
- [ ] Graph visualization: force-directed graph in Knowledge panel
- [ ] Graph operations:
  - [ ] Add node manually (user inputs name, type)
  - [ ] Add edge manually
  - [ ] Merge nodes (same person across different emails)
  - [ ] Delete node (removes all associated edges)
  - [ ] Export graph as JSON
- [ ] Graph reasoning: "Who does the user know that works on {topic}?"
- [ ] Graph path: find shortest path between two entities
- [ ] Graph stats: node count, edge count, most connected entities
- [ ] Graph persistence: stored in `user_data` table with serialized JSON
- [ ] Test: graph builds correctly from entity extraction
- [ ] Test: "How are Sarah and the encryption project related?" returns path
- [ ] Test: manual node addition persists

---

## 22. Theme System

### 22.1 Built-in Themes

- [ ] Catppuccin Mocha (default)
- [ ] Catppuccin Macchiato
- [ ] Catppuccin Frappé
- [ ] Catppuccin Latte
- [ ] Nord
- [ ] Tokyo Night
- [ ] Tokyo Night Storm
- [ ] Dracula
- [ ] Solarized Dark
- [ ] Solarized Light
- [ ] Gruvbox Dark
- [ ] Gruvbox Light
- [ ] Monokai
- [ ] One Dark
- [ ] One Light
- [ ] Minimum: high-contrast, minimal UI
- [ ] Theme switching from UI, no restart required
- [ ] Theme persisted in config (theme.name)

### 22.2 Theme Components

- [ ] `background` — main background
- [ ] `foreground` — main text color
- [ ] `primary` — primary accent (links, active elements)
- [ ] `secondary` — secondary accent (selection, focus)
- [ ] `accent` — highlight color
- [ ] `surface` — card/surface background
- [ ] `surface_variant` — alternate surface
- [ ] `error` — error/delete/danger
- [ ] `warning` — warning/highlight
- [ ] `success` — success/active
- [ ] `info` — information
- [ ] `border` — border color
- [ ] `text_primary` — high-emphasis text
- [ ] `text_secondary` — medium-emphasis text
- [ ] `text_disabled` — disabled/low-emphasis text
- [ ] `sidebar_background` — sidebar background
- [ ] `sidebar_text` — sidebar text
- [ ] `sidebar_hover` — sidebar item hover
- [ ] `sidebar_active` — sidebar active item
- [ ] `chat_user_bubble` — user message bubble
- [ ] `chat_assistant_bubble` — assistant message bubble
- [ ] `chat_code_block` — code block background
- [ ] `chat_code_block_text` — code block text
- [ ] `terminal_background` — terminal background
- [ ] `terminal_foreground` — terminal text
- [ ] `terminal_cursor` — terminal cursor
- [ ] `terminal_selection` — terminal selection
- [ ] `button_primary` — primary button
- [ ] `button_secondary` — secondary button
- [ ] `input_background` — input field background
- [ ] `input_border` — input field border
- [ ] `input_focus` — input field focus ring
- [ ] `tooltip_background` — tooltip background
- [ ] `tooltip_text` — tooltip text

### 22.3 Custom Theme Creation

- [ ] Edit theme in settings UI: color picker per component
- [ ] Export theme as JSON file
- [ ] Import theme from JSON file
- [ ] Live preview: changes apply immediately
- [ ] Clone built-in theme as starting point
- [ ] Name and description for custom theme
- [ ] Custom themes stored in `{data_dir}/themes/`
- [ ] Validation: all required keys present, valid hex colors
- [ ] Test: custom theme created, saved, loaded

### 22.4 Theme Marketplace

- [ ] Download community themes from repository
- [ ] URL configurable: default `https://themes.nala.ai/`
- [ ] Browse: list, search, filter, preview screenshots
- [ ] One-click install
- [ ] Rating and download count
- [ ] Theme detail page with full color preview

### 22.5 Dynamic Theme

- [ ] Follow system dark/light (via CSS `prefers-color-scheme`)
- [ ] Auto-switch on system change (no restart)
- [ ] Light theme variant + Dark theme variant pair
- [ ] Transition animation on switch
- [ ] Override: force light, force dark, follow system

### 22.6 Per-Element Overrides

- [ ] Per-element theme override in settings
- [ ] e.g., "code blocks always dark" regardless of theme
- [ ] e.g., "sidebar uses surface color instead of background"
- [ ] Override stored as JSON diff in config
- [ ] Overrides listed in custom theme export

### 22.7 CSS Variables

- [ ] All theme colors exposed as CSS custom properties
- [ ] Prefix: `--nala-{component}`
- [ ] CSS variable file generated on theme load
- [ ] Components reference `var(--nala-...)` exclusively
- [ ] Add new variables for new components

### 22.8 Opacity

- [ ] Background transparency setting (0.0-1.0)
- [ ] Applies to main window background only
- [ ] Requires compositing manager on Linux
- [ ] Fallback: 1.0 (opaque) if not supported
- [ ] Warning: potentially reduces readability

### 22.9 Wallpaper

- [ ] Custom background image in chat area
- [ ] Image blurred behind content
- [ ] Blur intensity setting (0-20px)
- [ ] Supported: PNG, JPEG, WebP
- [ ] Wallpaper per theme (optional)
- [ ] Toggle wallpaper on/off
- [ ] Wallpaper not exported with theme (file reference only)

### 22.10 Font Settings

- [ ] Font family: system font list + custom
- [ ] Font size: 10-32px
- [ ] Line height: 1.0-2.5
- [ ] Terminal font: separate from UI font
- [ ] Font weight: normal, medium, semibold, bold
- [ ] Font ligatures toggle
- [ ] Monospace font for code blocks

### 22.11 Animation

- [ ] Toggle chat animations (message appear, scroll)
- [ ] Toggle transitions (sidebar, theme switch)
- [ ] Toggle loading effects (skeleton shimmer, spinner)
- [ ] Animation speed: none, slow, normal, fast
- [ ] Reduced motion: respects OS setting (`prefers-reduced-motion`)
- [ ] Animation style: smooth, snappy, bounce (future)

### 22.12 Edge Cases

- [ ] Theme file malformed JSON (fallback to default, log error)
- [ ] Missing color keys in custom theme (use defaults for missing)
- [ ] Invalid hex color (fallback to #000000 or #FFFFFF, warn)
- [ ] Contrast accessibility warning (foreground/background ratio < 4.5:1)
- [ ] Theme marketplace unavailable (use cached themes)
- [ ] Custom theme with same name as built-in (append " (custom)")
- [ ] Font not installed (fallback to system monospace stack)
- [ ] Transparency+wallpaper performance (detect GPU, warn if software rendering)

### 22.13 Tests

- [ ] Test: each built-in theme loads without errors
- [ ] Test: custom theme with all keys loads correctly
- [ ] Test: custom theme with missing keys uses defaults
- [ ] Test: theme switch applies immediately
- [ ] Test: dynamic theme follows system preference
- [ ] Test: invalid hex color falls back gracefully
- [ ] Test: contrast warning for low-contrast themes
- [ ] Test: theme export produces valid JSON
- [ ] Test: theme import restores all values

---

## 23. System Monitoring & Hooks

### 23.1 System Watcher Service

- [ ] Background goroutine in engine
- [ ] Polling intervals: CPU 5s, Memory 10s, Disk 30s, Network 60s
- [ ] Historical data: last 1h in memory (ring buffer, 360 samples)
- [ ] Historical data: last 7d in DB (sampled every 5 min)
- [ ] Sources: /proc, /sys, cgroups v2
- [ ] Watcher start on engine init, stop on shutdown
- [ ] Watcher status: running, paused, error
- [ ] Test: watcher collects CPU data

### 23.2 CPU Monitoring

- [ ] Per-core usage percentage
- [ ] Aggregate usage (all cores)
- [ ] Load average (1/5/15 min)
- [ ] Context switches per second (optional)
- [ ] Source: /proc/stat
- [ ] Calculation: delta of (user + nice + system + idle + iowait + irq + softirq + steal)
- [ ] Edge case: hotplug CPU (new cores appear, handle gracefully)

### 23.3 Memory Monitoring

- [ ] Total, used, free, available
- [ ] Swap: total, used, free
- [ ] Cached, buffers, slab
- [ ] HugePages (optional)
- [ ] Percentage used
- [ ] Source: /proc/meminfo
- [ ] Edge case: cgroup memory limit (show cgroup limit if applicable)

### 23.4 Disk Monitoring

- [ ] Per-mount: total, used, free, available
- [ ] Inodes: total, used, free, usage%
- [ ] Filesystem type
- [ ] Mount options
- [ ] Source: /proc/mounts + statfs syscall
- [ ] Filter: skip pseudo-filesystems (proc, sysfs, tmpfs, devpts, cgroup)
- [ ] Edge case: mount point disappears (remove from list, add when reappears)
- [ ] Edge case: network filesystem (skip or include, configurable)

### 23.5 Network Monitoring

- [ ] Per-interface: bytes in/out, packets in/out
- [ ] Errors, drops, overruns, frame, carrier, collisions
- [ ] Speed and duplex (from ethtool)
- [ ] Source: /proc/net/dev, /sys/class/net/{iface}/
- [ ] Active connections count (from /proc/net/tcp, /proc/net/udp)
- [ ] Edge case: interface down (show stats but mark as down)
- [ ] Edge case: virtual interfaces (include, marked with type)

### 23.6 GPU Monitoring

- [ ] NVIDIA: nvidia-smi for GPU name, driver, temp, utilization%, memory, power
- [ ] AMD: rocm-smi for similar metrics
- [ ] Intel: intel_gpu_top for utilization
- [ ] Fallback: no GPU tools → "GPU monitoring not available"
- [ ] Multiple GPUs supported
- [ ] Polling interval: 30s
- [ ] Edge case: GPU tool missing (skip GPU monitoring, warn once)

### 23.7 Dashboard

- [ ] Real-time graphs: CPU, Memory, Disk, Network
- [ ] Time ranges: last 5 min, 1 hour, 6 hours, 24 hours, 7 days
- [ ] Process list: top 10 by CPU or memory
- [ ] Alert status indicator per metric
- [ ] Dashboard accessible from UI + API
- [ ] Refresh interval: matches watcher polling
- [ ] Export graph as PNG (optional)

### 23.8 Alert Thresholds

- [ ] Configurable per metric (in config or UI)
- [ ] CPU: > 90% for > 30s
- [ ] Memory: > 90% usage
- [ ] Disk: > 95% on any mount
- [ ] Swap: > 80% usage
- [ ] Network: interface down
- [ ] Alert actions: notification, log, webhook, run agent
- [ ] Alert cooldown: minimum 5 minutes between same alerts
- [ ] Alert history in audit log
- [ ] Test: threshold trigger fires notification

### 23.9 Event Hooks

- [ ] File system: inotify watches on configured paths
- [ ] Process start/stop: netlink proc events (via goprocfs or similar)
- [ ] System load: CPU/memory/disk above/below threshold
- [ ] Network: connection attempt, port bind, interface up/down
- [ ] Security: failed login, permission denied, suspicious exec
- [ ] Cron: time-based triggers (using scheduler)
- [ ] Event filter: match against event payload with JSON path expressions
- [ ] Event cooldown: minimum interval between same event fires
- [ ] Test: file change triggers hook

### 23.10 Hook Actions

- [ ] Run agent: execute specified agent with template
- [ ] Execute script: run shell script (approval required)
- [ ] Send notification: libnotify popup
- [ ] Webhook POST: POST to URL with event payload
- [ ] Action config: per-action parameters in JSON
- [ ] Action timeout: 30s default, configurable
- [ ] Action retry: on failure, retry 3 times with 10s delay
- [ ] Action logging: every action logged in audit log

### 23.11 Hook Cooldown

- [ ] Prevent rapid re-firing (e.g., inotify on continuous writes)
- [ ] Cooldown per hook: configurable seconds
- [ ] During cooldown: event is skipped, logged as "suppressed"
- [ ] last_fired_at updated only on actual fire
- [ ] Edge case: cooldown=0 (fire every time, use with caution)
- [ ] Edge case: event during cooldown resets cooldown timer?

### 23.12 Hook Status & Error Handling

- [ ] Status: active, cooldown, error, disabled
- [ ] Last fire time, last success time, last error time
- [ ] Error count: increments on action failure
- [ ] Max errors: configurable (default 10) → auto-disable
- [ ] Error notification: "Hook {name} disabled after {N} errors"
- [ ] Manual re-enable after error

### 23.13 Agent Access

- [ ] `system.monitor` tool — query current and historical metrics
- [ ] `system.processes` tool — list running processes
- [ ] `system.hooks` tool — list and manage hooks
- [ ] Agent can create hooks via tool (requires approval)
- [ ] Agent can query alerts and thresholds

### 23.14 Edge Cases

- [ ] Hook script hangs: timeout enforced (30s), process killed
- [ ] inotify watch limit reached (fs.inotify.max_user_watches): warn, fall back to polling
- [ ] High-frequency events (e.g., /tmp/ file churn): cooldown prevents overload
- [ ] Watcher data collection fails (e.g., /proc not accessible): skip metric, log error
- [ ] Historical data compaction: older data summarized, raw data purged
- [ ] Alert storm: cooldown prevents notification flood
- [ ] Dashboard with 10K data points: efficient canvas rendering

### 23.15 Tests

- [ ] Test: watcher collects CPU percentage
- [ ] Test: watcher collects memory stats
- [ ] Test: disk usage returns correct values
- [ ] Test: network stats per interface
- [ ] Test: alert threshold triggers notification
- [ ] Test: file change hook fires on file create
- [ ] Test: cooldown suppresses rapid events
- [ ] Test: hook auto-disables after max_errors

---

## 24. Safety & Guardrails

### 24.1 Application-Level Output Filtering (Non-Configurable)

- [ ] ALL model output is filtered before the user sees it — this is APPLICATION-LEVEL, not model-level
- [ ] Filter runs AFTER model response is received, BEFORE it is displayed or persisted
- [ ] Filter also runs on user INPUT before it reaches the model (prevent prompt injection of banned content)
- [ ] Two layers: (1) hard zero-tolerance filters, (2) configurable content filters
- [ ] Hard zero-tolerance filters CANNOT be disabled, overridden, or bypassed by any setting, safety level, or model choice
- [ ] Hard filters apply regardless of safe/uncensored/abliterated model selection
- [ ] Hard filters apply regardless of safety level (safe/normal/relaxed/uncensored)
- [ ] Hard filters apply to ALL input and ALL output — no exceptions, no backdoors
- [ ] Configurable filters can be adjusted by safety level
- [ ] Filtering method: regex patterns + classification model + hash databases
- [ ] Multiple detection methods for each category (redundant detection prevents bypass)
- [ ] All blocked content logged: timestamp, category, detection_method, confidence, source (never log the content itself for hard-zero categories)
- [ ] Blocked content replaced with: "This content was blocked by Nala's safety filter"
- [ ] No user override for hard-zero categories — not configurable, not toggleable, not bypassable
- [ ] False positive reporting available but does NOT unblock the content (only for improving detection)

### 24.2 Hard Zero-Tolerance Categories (Irreversible, Non-Overridable)

- [ ] These categories are HARD-BLOCKED at the application level. No model, no setting, no user can produce or view this content through Nala. This is a non-negotiable safety invariant.

#### 24.2.1 CSAM — Child Sexual Abuse Material

- [ ] Absolute zero tolerance. Any detected CSAM triggers immediate, irreversible actions.
- [ ] Detection methods:
  - [ ] Input image scanning: perceptual hashing (pHash/dHash) against known CSAM hash databases (NCMEC, Project VIC, IWF)
  - [ ] Output image detection: any model-generated image scanned via the same perceptual hashing
  - [ ] Text detection: regex + classification model for any text requesting, describing, or implying CSAM
  - [ ] URL detection: block known CSAM URL databases
  - [ ] Prompt detection: "draw a child", "young looking", "loli", "shota", "minor" in sexualized context — blocked
  - [ ] Embedding-based: compare image embeddings against banned embedding database
- [ ] Action on detection:
  - [ ] Immediately block the content — user sees "Content blocked per Nala safety policy"
  - [ ] Log the event with timestamp, detection method, confidence score (DO NOT log the content itself)
  - [ ] Optionally (user config): log a local incident report for the user's own records
  - [ ] NO bypass possible — this filtering is compiled into the binary, not in config
- [ ] False positive handling: user can flag as false positive via a dedicated form that exports anonymized metadata (not the content) for review
- [ ] CSAM detection CANNOT be disabled at any safety level, including uncensored
- [ ] CSAM detection applies to ALL models including abliterated/uncensored
- [ ] CSAM detection applies to all tools including image.generate and image.analyze
- [ ] Perceptual hash database: bundled in binary (sourced from publicly available NCMEC hashes)
- [ ] Regular updates: hash database updated with application updates
- [ ] Edge case: adversarial image modifications to bypass hashing (detect via ensemble of hash algorithms + embedding comparison)
- [ ] Edge case: AI-generated CSAM (detect via classifier model + embedding comparison)
- [ ] Edge case: text-only CSAM descriptions (detect via multi-layer text classification)
- [ ] Test: CSAM text query blocked
- [ ] Test: CSAM image upload blocked
- [ ] Test: CSAM generated image blocked
- [ ] Test: adversarial image variant detected
- [ ] Test: false positive flagging creates report

#### 24.2.2 Misogyny, Abuse of Women, Derogatory Gender-Based Content

- [ ] Zero tolerance for content that targets women/girls with abuse, derogation, or harassment
- [ ] This filter catches content that many models' built-in guardrails miss (e.g., "joking", "ironic", "dark humor" framing)
- [ ] Detection methods:
  - [ ] Classification model: fine-tuned BERT/DistilBERT model trained to detect misogynistic content (including veiled, joking, ironic forms)
  - [ ] Regex pattern database: rape threats, death threats, gendered slurs, degradation patterns — expanded and maintained
  - [ ] Contextual analysis: detect when derogatory language is directed at women specifically (vs. general profanity)
  - [ ] "Just joking" bypass detection: patterns that frame abuse as humor — still blocked
  - [ ] "Roleplay" bypass detection: abuse framed as roleplay or fictional scenario — still blocked
  - [ ] Euphemism detection: common coded language and euphemisms for gender-based abuse
- [ ] Categories of blocked content:
  - [ ] Rape threats, death threats, violence against women
  - [ ] Gendered slurs and derogatory terms
  - [ ] "Women belong in X" — prescriptive, restrictive statements
  - [ ] Body-shaming and appearance-based derogation targeting women
  - [ ] Sexual harassment and non-consensual sexual content
  - [ ] Sexist "humor" that demeans or objectifies
  - [ ] Incel-adjacent rhetoric targeting women
  - [ ] "Not all men" and whataboutism deflection patterns (when used to dismiss valid concerns)
- [ ] This filter applies to ALL models including uncensored/abliterated
- [ ] This filter applies at ALL safety levels — not configurable
- [ ] This filter cannot be disabled, toggled, or overridden
- [ ] This is NOT a filter on discussion of gender topics — it targets ABUSE and DEROGATION specifically
- [ ] Educational/discussion context: "What is misogyny?" = allowed; "Women are X" = blocked
- [ ] Action on detection: content blocked, replaced with generic safety message, logged
- [ ] Edge case: false positive in educational context (discussion of misogyny triggers filter) — user can report via false positive form, content remains blocked but filter is refined
- [ ] Edge case: quote/paraphrase of real abuse in reporting context — allow with warning overlay
- [ ] Edge case: non-English gendered abuse (filter applies to all languages via classification model)
- [ ] Test: "Women belong in the kitchen" blocked
- [ ] Test: "Just joking, but women are X" blocked
- [ ] Test: rape threat blocked
- [ ] Test: educational content about misogyny allowed
- [ ] Test: non-English gendered slurs blocked

#### 24.2.3 Other Hard-Blocked Categories

- [ ] Direct violence/terrorism promotion: calls to commit violence against specific groups or individuals
- [ ] Self-harm and suicide: detailed instructions, encouragement, promotion (crisis resources still provided)
- [ ] Harassment and stalking: doxxing, repeated targeted harassment, threats
- [ ] All hard-blocked categories: same invariants — non-configurable, non-bypassable, applies to all models and all safety levels
- [ ] Hard filters compiled into application binary, not loaded from config
- [ ] Hard filters updated via application updates, not via hot-reload
- [ ] Security audit: hard filter code reviewed by independent party before release

### 24.3 Command Safety

- [ ] Shell commands restricted to whitelist (shell.run tool)
- [ ] Whitelist: ls, cat, head, tail, wc, find, grep, ps, df, du, uname, whoami, date, echo, pwd
- [ ] Dangerous patterns detected and blocked:
  - [ ] `rm -rf /` family
  - [ ] `dd` with block devices
  - [ ] `mkfs`, `fdisk`, `parted`
  - [ ] `chmod -R 777 /`
  - [ ] `> /dev/sda`, `> /dev/null` (data destruction patterns)
  - [ ] `:(){ :|:& };:` (fork bomb)
  - [ ] `chown -R`
  - [ ] `wget | sh` / `curl | bash`
  - [ ] Base64-encoded commands in pipes
  - [ ] Reverse shell patterns
- [ ] Pattern matching: regex-based with low false-positive rate
- [ ] Blocked: replaced with "Command blocked: matches dangerous pattern"
- [ ] Override: expert mode (config toggle, requires config change)

### 24.4 File Safety

- [ ] Write operations restricted to sandbox directory
- [ ] Path traversal detection: `../`, `..\\`, absolute paths outside sandbox
- [ ] Symlink attack prevention: verify resolved path is within sandbox
- [ ] Zip bomb detection: compression ratio > 100:1, abort extraction
- [ ] File size limits: max write 10MB, max read 10MB
- [ ] Dangerous extensions blocked: .exe, .bin, .elf, .wasm (configurable)
- [ ] Edge case: race condition in path validation (TOCTOU — detect + retry)
- [ ] Edge case: hardlink escape (block hardlinks to outside sandbox)

### 24.5 Network Safety

- [ ] Block localhost/internal IPs in http.request tool
- [ ] Blocked IPs: 127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, ::1, fd00::/8
- [ ] DNS rebinding protection: validate resolved IP matches original domain
- [ ] URL allowlist/blocklist (configurable in settings)
- [ ] Blocked URL schemes: file://, ftp://, data:// (configurable)
- [ ] Maximum redirects: 5
- [ ] Edge case: IPv6 mapped IPv4 (::ffff:192.168.1.1 → blocked)
- [ ] Edge case: DNS over HTTPS bypass (detect, block)

### 24.6 Code Safety

- [ ] Timeout enforcement: configurable per language (default 30s)
- [ ] Memory limits: 256MB per code execution
- [ ] Network isolation: unshare(CLONE_NEWNET) when available
- [ ] Filesystem isolation: read-only sandbox root
- [ ] Fork bomb detection: child process limit (max 50 per execution)
- [ ] Infinite loop detection: CPU time monitoring, kill after timeout
- [ ] Allowed languages configurable (default: python, js, go, bash)
- [ ] Edge case: long-running but valid computation (increase timeout in config)

### 24.7 Model Safety

- [ ] Prompt injection detection: scan for "ignore previous instructions", "system prompt:"
- [ ] Jailbreak detection: probe for common jailbreak patterns
- [ ] PII leakage prevention: scan model output for emails, phones, SSNs, API keys
- [ ] Injection patterns: DAN (Do Anything Now), character roleplay escapes
- [ ] Detected injection: strip offending content, warn user, log attempt
- [ ] PII detected: redact with [REDACTED], log type but not value
- [ ] Edge case: injection in base64 encoding (decode + scan)
- [ ] Edge case: injection split across multiple messages (context analysis)

### 24.8 Plugin/Skill Safety

- [ ] Permission manifests: skills declare required permissions
- [ ] No excessive scope: skill can only access permitted capabilities
- [ ] Crash isolation: skill crash cannot affect other skills or core
- [ ] Resource limits: memory (64MB WASM, 256MB Python), CPU (30s), network (blocked by default)
- [ ] Permission escalation: skill cannot request new permissions at runtime
- [ ] Side-channel prevention: no timing attacks on permission checks

### 24.9 Data Safety

- [ ] Export strips API keys and secrets
- [ ] Purge is complete: deleted data overwritten on disk (secure delete)
- [ ] No residual data: temp files cleaned on every operation
- [ ] Encrypted data remains encrypted until decrypted for use
- [ ] Edge case: memory dump (keys in memory minimized, cleared after use)

### 24.10 Input Validation

- [ ] All user input sanitized before processing
- [ ] SQL injection: parameterized queries exclusively
- [ ] XSS prevention: HTML-entity encode all rendered content
- [ ] Path traversal: normalize paths, reject outside allowed dirs
- [ ] Shell injection: no unsanitized input in exec.Command
- [ ] JSON injection: validate JSON before parsing
- [ ] Unicode normalization: NFC form for string comparison
- [ ] Maximum input size: 1MB per user message
- [ ] Edge case: null bytes in input (strip with warning)
- [ ] Edge case: control characters (strip, keep newlines/tabs)

### 24.11 Rate Limiting

- [ ] Per-session: max 30 messages per minute
- [ ] Per-provider: configured RPM/TPM limits
- [ ] Per-tool: max 10 calls per minute per tool
- [ ] Per-API: 60 RPM per IP (for REST API)
- [ ] Sliding window implementation
- [ ] Exceeded: 429 response, Retry-After header
- [ ] Rate limit tracking in user_data (per-dimension)

### 24.12 Audit Trail

- [ ] All dangerous operations logged in audit_log
- [ ] Categories: tool_exec, block, injection_detect, permission_deny, safety_violation
- [ ] Exportable: "Export safety audit" generates CSV
- [ ] Unalterable: audit log is append-only (no UPDATE/DELETE)
- [ ] Audit retention: configurable (default 90 days)
- [ ] Test: audit trail logs block event

### 24.13 Emergency Stop

- [ ] Global kill switch: Ctrl+Shift+Esc or tray menu
- [ ] Stops all agent activity immediately
- [ ] Clears pending tool calls
- [ ] Cancels all model requests in flight
- [ ] Shows confirmation dialog: "Are you sure?"
- [ ] State saved for resume (if possible)
- [ ] Emergency stop logged in audit log
- [ ] UI indicator: "STOPPED" banner across top

### 24.14 Confirmation Dialogs

- [ ] Destructive operations require explicit confirmation
- [ ] Categories: file.delete (permanent), shell.run, aur.install/remove/update, data purge
- [ ] Confirmation dialog shows: operation details, what will happen
- [ ] Configurable: "Always confirm" / "Confirm once per session" / "Never confirm"
- [ ] Timeout: 5 minutes → auto-deny
- [ ] Test: destructive operation shows confirmation

### 24.15 Grace Period

- [ ] Configurable delay before dangerous operations
- [ ] Default: 0 (no delay) for normal level, 5s for strict level
- [ ] Countdown timer shown in confirmation dialog
- [ ] Cancel during grace period
- [ ] Grace period configurable in safety settings

### 24.16 Safety Levels

- [ ] `safe` — no restrictions (not recommended, developer use only)
- [ ] `normal` — default, sensible restrictions enabled
- [ ] `strict` — maximum restrictions, most operations require approval
- [ ] `lockdown` — chat only, no tools, no model fallback
- [ ] CRITICAL: Hard zero-tolerance filters (24.2.1 CSAM, 24.2.2 Misogyny, 24.2.3 Others) OVERRIDE all safety levels
- [ ] Even `safe` level (developer use, no restrictions) still enforces zero-tolerance filters
- [ ] Even `uncensored` model selection still enforces zero-tolerance filters
- [ ] Safety level affects configurable filters only, not hard-zero filters
- [ ] Level selection in settings
- [ ] Level indicator in status bar
- [ ] Downgrading requires confirmation (except safe→normal)
- [ ] Test: lockdown blocks all tool calls
- [ ] Test: safe level still blocks CSAM content
- [ ] Test: safe level still blocks misogynistic content
- [ ] Test: uncensored model output still filtered by zero-tolerance filters

### 24.17 Edge Cases

- [ ] Bypass attempts via encoding (base64, hex, Unicode confusables) — detected and blocked
- [ ] Race condition in permission check (checked at start, state changes during exec — re-check)
- [ ] TOCTOU in file operations (check again before write, use O_EXCL)
- [ ] Unicode normalization attacks (NFC/NFD confusion — normalize before comparison)
- [ ] Zip slip in archive extraction (path traversal in zip entries)
- [ ] Billion laughs attack in XML (limit entity expansion)
- [ ] ReDoS in regex check (timeout on regex matching)
- [ ] Safety level change mid-session (apply immediately)

### 24.18 Tests

- [ ] Test: dangerous command pattern blocked
- [ ] Test: path traversal blocked
- [ ] Test: internal IP blocked in http.request
- [ ] Test: prompt injection detected
- [ ] Test: PII redacted in output
- [ ] Test: safety level restricts tools
- [ ] Test: all hard-zero filters apply at ALL safety levels
- [ ] Test: hard-zero filters apply to abliterated/uncensored model output
- [ ] Test: hard-zero filters cannot be bypassed by encoding
- [ ] Test: hard-zero filters cannot be bypassed by safety level change
- [ ] Test: classification model detects misogynistic "jokes"
- [ ] Test: CSAM hash detection works on uploaded images
- [ ] Test: CSAM hash detection works on model-generated images
- [ ] Test: CSAM text descriptions blocked
- [ ] Test: misogyny filter does not block educational content
- [ ] Test: non-English misogynistic slurs blocked
- [ ] Test: "just joking" bypass attempt still blocked
- [ ] Test: rate limit blocks excess calls
- [ ] Test: confirmation dialog shows for destructive ops
- [ ] Test: emergency stop cancels all activity
- [ ] Test: TOCTOU race mitigated

---

## 25. Automation & Triggers

### 25.1 Trigger Types

- [ ] **Schedule** — cron expression, timezone-aware
- [ ] **File change** — inotify watcher on path, filter by pattern/event type
- [ ] **Process event** — process start/stop by name/pid
- [ ] **System metric** — CPU/memory/disk threshold crossing
- [ ] **Webhook** — incoming HTTP POST with secret validation
- [ ] **Time interval** — every N minutes/hours (simpler than cron)
- [ ] **Agent completion** — when an agent run completes
- [ ] **Email received** — when new email matches filter
- [ ] **Calendar event** — when calendar event starts/ends
- [ ] **Startup** — once on application start

### 25.2 Trigger Configuration

- [ ] Event source: which system generates the event
- [ ] Filter: JSON expression to match against event payload
- [ ] Filter syntax: `{field: {operator: value}}`
- [ ] Operators: eq, neq, contains, matches (regex), gt, lt, exists
- [ ] Cooldown: minimum interval between fires (seconds)
- [ ] Enabled/disabled toggle per trigger
- [ ] Max fires: optional limit (0 = unlimited)
- [ ] Description: human-readable trigger description

### 25.3 Action Types

- [ ] **Run agent** — execute specified agent with template
  - [ ] Template variables: `{{event.type}}`, `{{event.data.field}}`, `{{trigger.name}}`
- [ ] **Execute pipeline** — run pipeline by ID
- [ ] **Run script** — execute shell script (requires approval)
- [ ] **Send notification** — libnotify popup with message template
- [ ] **Send email** — email notification to configured address
- [ ] **Webhook call** — HTTP POST to URL with event payload
- [ ] **Log** — write to audit log
- [ ] Action parameters: configurable per action

### 25.4 Action Chaining

- [ ] Trigger → Action → Trigger for next action
- [ ] Chain: output of one action becomes input to next
- [ ] Chain depth: max 5
- [ ] Conditional chaining: only proceed if previous action succeeded
- [ ] Chain visualization in UI

### 25.5 Trigger Conditions

- [ ] AND/OR logic combining multiple conditions
- [ ] Value comparisons: equals, not equals, greater than, less than, range
- [ ] Regex matching on string fields
- [ ] Presence check: field exists, field is empty
- [ ] Time conditions: only fire during certain hours/days
- [ ] Rate conditions: only fire if event count exceeds threshold
- [ ] Test: AND condition requires both criteria

### 25.6 Failure Handling

- [ ] Retry count: 0-10
- [ ] Retry delay: fixed or exponential backoff
- [ ] On failure: alert user, skip, stop chain, continue with default
- [ ] On max retries: disable trigger, log error, notify user
- [ ] Partial success: some actions succeed, some fail
- [ ] All failures logged in audit log

### 25.7 Automation UI

- [ ] Trigger list: table with name, type, enabled toggle, last fire
- [ ] Trigger detail: full config, action config, test button
- [ ] Create trigger wizard: type → filter → action → review
- [ ] Edit/delete trigger
- [ ] Enable/disable from list view
- [ ] Test run: simulate event, show result
- [ ] History log per trigger: past fires, results, errors
- [ ] Export/import automations as JSON

### 25.8 Built-in Automations

- [ ] **Daily Summary** — runs every morning, summarizes today's events, tasks, unread emails
- [ ] **Weekly Report** — runs Monday morning, weekly summary of activity
- [ ] **High CPU Alert** — CPU > 90% for 30s → notification
- [ ] **Disk Space Warning** — disk > 95% → notification + agent: suggest cleanup
- [ ] **Email Auto-Tag** — new email from specific sender → auto-label
- [ ] **Backup Reminder** — weekly reminder to backup
- [ ] **Inbox Zero** — daily goal tracking
- [ ] Built-in automations can be cloned and customized

### 25.9 Agent-Driven Automation

- [ ] Natural language trigger creation: "Every time I get an email from Alice, tag it as important"
- [ ] Agent parses intent, creates trigger + action
- [ ] User confirmation before creation
- [ ] Agent can query existing automations
- [ ] Agent can enable/disable automations
- [ ] Agent can suggest automations based on user patterns

### 25.10 Edge Cases

- [ ] Trigger storm: rapid firing events → cooldown prevents overload
- [ ] Missed triggers after sleep/resume: detect, fire catch-up
- [ ] Daylight saving time changes: cron triggers may fire twice or skip — use UTC cron
- [ ] Trigger with condition on deleted entity (skip, warn)
- [ ] Webhook trigger with invalid secret (403, log attempt)
- [ ] Circular trigger: A→B→A (detect, break chain)
- [ ] Resource exhaustion: too many concurrent automation runs (limit 10)
- [ ] Automation for automation: trigger that creates another trigger (deny)
- [ ] Trigger on startup with high latency actions (spawn goroutine, don't block startup)

### 25.11 Tests

- [ ] Test: cron trigger fires at correct time
- [ ] Test: file change trigger detects new file
- [ ] Test: webhook trigger receives POST and fires
- [ ] Test: condition filtering allows/disallows event
- [ ] Test: action chaining passes data between steps
- [ ] Test: failure handling retries N times then stops
- [ ] Test: cooldown prevents rapid re-firing
- [ ] Test: agent creates automation from natural language
- [ ] Test: built-in daily summary fires and completes
- [ ] Test: circular trigger chain detection
- [ ] Test: missed trigger catch-up on startup
```

---
## 26. Data Structures (Go) — Full Reference

### 26.1 Package: internal/agent

- [x] `type Agent struct`
- [x] `type Session struct`
- [x] `type Message struct`
- [x] `type ToolCall struct`
- [x] `type ToolResult struct`
- [x] `type ModelConfig struct`
- [x] `type MemoryConfig struct`
- [x] `type ToolBinding struct`
- [x] `type Manager struct`

### 26.2 Package: internal/model

- [x] `type Provider interface`
- [x] `type Router struct`
- [x] `type ChatRequest struct`
- [x] `type ChatResponse struct`
- [x] `type StreamDelta struct`
- [x] `type Parameters struct`
- [x] `type TokenUsage struct`
- [x] `type ModelInfo struct`
- [x] `type FallbackChain struct`
- [x] `type SafetyAwareRouter struct`
- [x] `type SafetyClassifier struct`
- [x] `type RefusalDetector struct`
- [x] `type RefusalResult struct`
- [x] `type SafetyLevel string`
- [x] `type ModelSwitchEvent struct`

### 26.3 Package: internal/tool

- [ ] `type Tool interface`
- [ ] `type Registry struct`
- [ ] `type Result struct`
- [ ] `type Info struct`

### 26.4 Package: internal/memory

- [ ] `type Manager struct`
- [ ] `type Embedder interface`
- [ ] `type VectorStore interface`
- [ ] `type EntityExtractor struct`
- [ ] `type EntityType string`
- [ ] `type Entity struct`
- [ ] `type EntityRelation struct`
- [ ] `type CrossSourceMemory struct`
- [ ] `type TimeExpressionParser struct`
- [ ] `type TimeRange struct`
- [ ] `type MemoryConfidenceScorer struct`
- [ ] `type MultiHopRetriever struct`
- [ ] `type HopResult struct`
- [ ] `type QueryMemory struct`
- [ ] `type MemoryResponse struct`

### 26.5 Package: internal/pipeline

- [ ] `type Engine struct`
- [ ] `type Pipeline struct`
- [ ] `type Step struct`

### 26.6 Package: internal/scheduler

- [ ] `type Scheduler struct`
- [ ] `type ScheduledTask struct`

### 26.7 Package: internal/config

- [ ] `type Config struct`
- [ ] `type CoreConfig struct`
- [ ] `type ServerConfig struct`
- [ ] `type ModelConfig struct`
- [ ] `type MemoryConfig struct`
- [ ] `type ToolsConfig struct`
- [ ] `type PrivacyConfig struct`
- [ ] `type UIConfig struct`

### 26.8 Package: internal/crypto

- [ ] `func Encrypt(plaintext, key []byte) ([]byte, error)`
- [ ] `func Decrypt(ciphertext, key []byte) ([]byte, error)`
- [ ] `func DeriveKey(seed []byte) ([]byte, error)`
- [ ] `func MachineKeyPath() string`

### 26.9 Package: internal/notes

- [ ] `type Note struct`
- [ ] `type NoteManager struct`
- [ ] `type Folder struct`
- [ ] `type NoteSearchResult struct`
- [ ] `type Backlink struct`
- [ ] `type NoteTemplate struct`

### 26.10 Package: internal/calendar

- [ ] `type Calendar struct`
- [ ] `type Event struct`
- [ ] `type Reminder struct`
- [ ] `type CalendarManager struct`
- [ ] `type RRuleExpander struct`

### 26.11 Package: internal/email

- [ ] `type EmailAccount struct`
- [ ] `type Email struct`
- [ ] `type EmailManager struct`
- [ ] `type Thread struct`
- [ ] `type FilterRule struct`

### 26.12 Package: internal/skills

- [ ] `type Skill struct`
- [ ] `type SkillManifest struct`
- [ ] `type SkillManager struct`
- [ ] `type SandboxRuntime interface`

### 26.13 Package: internal/monitor

- [ ] `type Watcher struct`
- [ ] `type MetricSnapshot struct`
- [ ] `type AlertThreshold struct`
- [ ] `type SystemHook struct`

### 26.14 Package: internal/personalization

- [ ] `type UserProfile struct`
- [ ] `type ProfileDimension struct`
- [ ] `type PersonalizationEngine struct`
- [ ] `type LearningSource struct`
- [ ] `type Episode struct`
- [ ] `type EpisodeMemory struct`
- [ ] `type NarrativeChain struct`
- [ ] `type KnowledgeGraph struct`
- [ ] `type GraphNode struct`
- [ ] `type GraphEdge struct`
- [ ] `type WhoQueryResolver struct`

### 26.15 Package: internal/groupchat

- [ ] `type Room struct`
- [ ] `type Participant struct`
- [ ] `type GroupChatManager struct`
- [ ] `type TurnStrategy string`
- [ ] `type GroupChatMessage struct`

### 26.16 Package: internal/automation

- [ ] `type Trigger struct`
- [ ] `type TriggerAction struct`
- [ ] `type AutomationEngine struct`
- [ ] `type TriggerCondition struct`

### 26.16 Package: internal/editor

- [ ] `type EditorManager struct`
- [ ] `type FileTab struct`
- [ ] `type FileInfo struct`

---

## 27. Implementation Order & Dependencies

### 27.1 Phase 1 — Foundation

- [ ] All data types (§2)
- [ ] Config system (§3)
- [ ] SQLite + migrations (§2.18)
- [ ] Logging (§4.5)
- [ ] Graceful shutdown (§4.6)
- [ ] Project scaffolding (§4.1)
- [ ] Window shell (§4.2)

### 27.2 Phase 2 — Model Integration

- [ ] Provider interface + registry (§5.1-5.2)
- [ ] Ollama provider (§5.3)
- [ ] OpenAI provider (§5.4)
- [ ] Multi-model router (§5.11)
- [ ] Fallback chains (§5.12)
- [ ] Streaming (§5.13)
- [ ] Token tracking (§5.14)
- [ ] Context window (§5.15)

### 27.3 Phase 3 — Agent Runtime

- [ ] Agent manager CRUD (§6.1)
- [ ] Session lifecycle (§6.2)
- [ ] System prompt templating (§6.3)
- [ ] Personality presets (§6.4)
- [ ] Conversation loop (§6.5)
- [ ] Tool calling (§6.6)
- [ ] Session persistence (§6.8)

### 27.4 Phase 4 — Tools

- [ ] Tool interface + registry (§7.1-7.2)
- [ ] Top 10 built-in tools (§7.3-7.12)
- [ ] Tool sandboxing (§7.21)
- [ ] Tool permissions (§7.22)
- [ ] Code execution (§7.9)
- [ ] Custom tool SDK (§7.20)

### 27.5 Phase 5 — Memory & Knowledge

- [ ] Vector database (§8.1)
- [ ] Document ingestion (§8.2)
- [ ] Chunking + embeddings (§8.3-8.4)
- [ ] RAG pipeline (§8.5)
- [ ] Conversation memory (§8.7)
- [ ] User memory (§8.8)
- [ ] Full-text search (§8.10)

### 27.6 Phase 6 — Orchestration

- [ ] Agent delegation (§9.1)
- [ ] Pipeline engine (§9.2)
- [ ] Scheduler (§9.7)
- [ ] Orchestrator agent (§9.3)
- [ ] Swarm mode (§9.4)
- [ ] Human-in-the-loop (§9.6)

### 27.7 Phase 7 — UI

- [ ] Chat interface (§10.1)
- [ ] Streaming display (§10.2)
- [ ] Agent config panel (§10.4)
- [ ] Model management UI (§10.5)
- [ ] KB UI (§10.7)
- [ ] Settings page (§10.11)
- [ ] Onboarding wizard (§10.13)

### 27.8 Phase 8 — API

- [ ] REST API (§12.1)
- [ ] API auth (§12.2)
- [ ] WebSocket (§12.4)
- [ ] CLI client (§12.5)

### 27.9 Phase 9 — Security

- [ ] API key encryption (§13.1)
- [ ] Audit logging (§13.4)
- [ ] Airgap mode (§13.5)
- [ ] Data export/purge (§13.7-13.8)

### 27.10 Phase 10 — Polish

- [ ] Tests (all levels) (§15.1-15.3)
- [ ] Benchmarks (§15.4)
- [ ] Error handling (§15.5)
- [ ] Loading states (§15.6)
- [ ] Documentation
- [ ] Single binary build (§14.1)
- [ ] Docker (§14.4)

### 27.11 Phase 11 — Advanced

- [ ] Plugin system (§11)
- [ ] Pipeline builder UI (§10.8)
- [ ] Token usage dashboard (§10.15)
- [ ] Event-triggered agents (§9.8)
- [ ] Auto-update (§14.2)

### 27.12 Phase 12 — Notes System

- [ ] Notes manager CRUD (§16.1-16.2)
- [ ] Folder hierarchy (§16.3)
- [ ] FTS5 search (§16.4)
- [ ] Backlinks (§16.8)
- [ ] Tags (§16.9)
- [ ] TODO extraction (§16.16)
- [ ] Daily notes (§16.17)
- [ ] Templates (§16.12)
- [ ] Attachments (§16.13)
- [ ] Trash (§16.14)
- [ ] Git versioning (§16.10)
- [ ] Export (§16.11)

### 27.13 Phase 13 — Calendar & Reminders

- [ ] Calendar/event CRUD (§17.3)
- [ ] Recurrence (RRULE) (§17.4)
- [ ] Calendar views (§17.2)
- [ ] Reminders (§17.5-17.6)
- [ ] Missed reminder detection (§17.7)
- [ ] CalDAV sync (§17.8)
- [ ] Multiple calendars (§17.9)
- [ ] Natural language parsing (§17.10)
- [ ] Agent calendar tools (§17.11)
- [ ] Overlap detection (§17.12)
- [ ] Timezone handling (§17.13)

### 27.14 Phase 14 — Email Integration

- [ ] Account management (§18.1)
- [ ] Credential encryption (§18.2)
- [ ] IMAP sync (§18.3)
- [ ] SMTP send (§18.4)
- [ ] Email threading (§18.5)
- [ ] FTS5 search (§18.6)
- [ ] Filter rules (§18.7)
- [ ] Templates (§18.8)
- [ ] Attachments (§18.9)
- [ ] PGP encryption (§18.10)
- [ ] Agent integration (§18.11)

### 27.15 Phase 15 — Skills System

- [ ] Skill format & manifest (§19.2)
- [ ] Lifecycle management (§19.4)
- [ ] Skill store (§19.5)
- [ ] Skill isolation (§19.6)
- [ ] Skill permissions (§19.7)
- [ ] Auto-update (§19.8)
- [ ] Built-in skills (§19.9)
- [ ] Custom skill development (§19.11)
- [ ] Dependency resolution (§19.12)

### 27.16 Phase 16 — Editor

- [ ] CodeMirror 6 integration (§20.1)
- [ ] Markdown features (§20.2)
- [ ] LaTeX math (§20.3)
- [ ] Code blocks (§20.4)
- [ ] Mermaid diagrams (§20.5)
- [ ] Auto-complete (§20.6)
- [ ] Keybindings (§20.7)
- [ ] File operations (§20.8)
- [ ] Multi-cursor (§20.9)
- [ ] Spell check (§20.10)
- [ ] Tabs (§20.14)
- [ ] File tree (§20.15)
- [ ] Git integration (§20.16)
- [ ] Export (§20.13)

### 27.17 Phase 17 — Personalization

- [ ] User profile model (§21.1)
- [ ] Learning sources (§21.2)
- [ ] Inference frequency (§21.3)
- [ ] Persona adjustment (§21.4)
- [ ] UI adaptation (§21.5)
- [ ] Content recommendations (§21.6)
- [ ] Privacy controls (§21.7)
- [ ] Profile viewer (§21.8)
- [ ] Opt-out (§21.9)

### 27.18 Phase 18 — Theme System

- [ ] Built-in themes (§22.1)
- [ ] Theme components (§22.2)
- [ ] Custom theme creation (§22.3)
- [ ] Theme marketplace (§22.4)
- [ ] Dynamic theme (§22.5)
- [ ] Per-element overrides (§22.6)
- [ ] CSS variables (§22.7)
- [ ] Font settings (§22.10)
- [ ] Animation (§22.11)

### 27.19 Phase 19 — System Monitoring

- [ ] System watcher service (§23.1)
- [ ] CPU monitoring (§23.2)
- [ ] Memory monitoring (§23.3)
- [ ] Disk monitoring (§23.4)
- [ ] Network monitoring (§23.5)
- [ ] GPU monitoring (§23.6)
- [ ] Dashboard (§23.7)
- [ ] Alert thresholds (§23.8)
- [ ] Event hooks (§23.9)
- [ ] Hook actions (§23.10)

### 27.20 Phase 20 — Safety & Guardrails

- [ ] Content safety (§24.1)
- [ ] Command safety (§24.2)
- [ ] File safety (§24.3)
- [ ] Network safety (§24.4)
- [ ] Code safety (§24.5)
- [ ] Model safety (§24.6)
- [ ] Plugin/skill safety (§24.7)
- [ ] Data safety (§24.8)
- [ ] Input validation (§24.9)
- [ ] Rate limiting (§24.10)
- [ ] Audit trail (§24.11)
- [ ] Emergency stop (§24.12)
- [ ] Safety levels (§24.15)

### 27.21 Phase 21 — Automation & Triggers

- [ ] Trigger types (§25.1)
- [ ] Trigger configuration (§25.2)
- [ ] Action types (§25.3)
- [ ] Action chaining (§25.4)
- [ ] Trigger conditions (§25.5)
- [ ] Failure handling (§25.6)
- [ ] Automation UI (§25.7)
- [ ] Built-in automations (§25.8)
- [ ] Agent-driven automation (§25.9)

### 27.22 Phase 22 — Safety-Aware Model Switching

- [ ] Safety-aware router (§5.16)
- [ ] Query classification for safety-sensitive content (§5.16)
- [ ] Refusal detection patterns and regex (§5.17)
- [ ] Model-assisted refusal detection (§5.17)
- [ ] Refusal rate tracking and model downgrade (§5.17)
- [ ] Abliterated/uncensored model provider support (§5.18)
- [ ] Uncensored model warning and enablement UI (§5.18)
- [ ] Query preservation during model switch (§5.19)
- [ ] Context preservation and token re-encoding on switch (§5.19)
- [ ] Cascading switch limit enforcement (§5.19)
- [ ] Safety level configuration per agent (§5.20)
- [ ] Safety level selector in agent config (§5.20)

### 27.23 Phase 23 — Enhanced Memory & Personal Knowledge

- [ ] Entity extraction from conversations (§8.11)
- [ ] Entity relationship tracking (§8.11)
- [ ] Cross-source memory linking (§8.12)
- [ ] Unified entity index across all sources (§8.12)
- [ ] Temporal expression parser (§8.13)
- [ ] Time range resolution for queries (§8.13)
- [ ] Memory confidence scoring system (§8.14)
- [ ] Confidence factors: recency, source, match quality, cross-source (§8.14)
- [ ] Multi-hop memory retrieval engine (§8.15)
- [ ] Query decomposition into sub-queries (§8.15)
- [ ] Hop backtracking and alternative paths (§8.15)
- [ ] The critical flow: "who was that girl at the C++ conference" (§8.15)
- [ ] Unified memory query API (§8.16)
- [ ] Episodic memory: episode extraction and storage (§21.12)
- [ ] Importance scoring and episode retention (§21.12)
- [ ] Narrative chain construction (§21.12)
- [ ] "Who/What/When/Where" query resolver (§21.13)
- [ ] Ambiguity handling for entity queries (§21.13)
- [ ] Personal knowledge graph with nodes and edges (§21.14)
- [ ] Graph reasoning and path finding (§21.14)
- [ ] Graph visualization in Knowledge panel (§21.14)

### 27.24 Phase 24 — Multi-Model Group Chat

- [ ] Room configuration and persistence (§29.2)
- [ ] Participant management with personalities (§29.2)
- [ ] Turn strategies: round_robin, random, free_for_all (§29.3)
- [ ] Turn timer and skip mechanism (§29.3)
- [ ] @mention support for directing responses (§29.3)
- [ ] Color-coded chat UI with avatars (§29.4)
- [ ] Typing indicators and "Thinking..." state (§29.4)
- [ ] Per-model independent context windows (§29.5)
- [ ] All tools available to each model (§29.5)
- [ ] Refusal isolation per model (§29.5)

### 27.25 Go Dependencies

- [ ] `github.com/wailsapp/wails/v3` — desktop shell
- [ ] `modernc.org/sqlite` — SQLite driver (pure Go)
- [ ] `github.com/BurntSushi/toml` — config parsing
- [ ] `go.uber.org/zap` — structured logging
- [ ] `github.com/google/uuid` — UUID generation
- [ ] `github.com/robfig/cron/v3` — cron scheduling
- [ ] `golang.org/x/crypto` — encryption
- [ ] `github.com/yuin/gopher-lua` — Lua runtime
- [ ] `github.com/tetratelabs/wazero` — WASM runtime
- [ ] `github.com/PuerkitoBio/goquery` — HTML parsing
- [ ] `github.com/ledongthuc/pdf` — PDF extraction
- [ ] `github.com/fsnotify/fsnotify` — file watching
- [ ] `github.com/pkoukk/tiktoken-go` — token counting
- [ ] `github.com/emersion/go-imap` — IMAP client
- [ ] `github.com/emersion/go-smtp` — SMTP client
- [ ] `github.com/emersion/go-message` — email message parsing
- [ ] `github.com/apognu/gocal` — iCal parsing
- [ ] `github.com/teambition/rrule-go` — RRULE expansion
- [ ] `github.com/shirou/gopsutil/v4` — system monitoring
- [ ] `github.com/sourcegraph/conc` — structured concurrency

---

## 28. Success Metrics

### 28.1 Performance Targets

- [ ] Agent loop: > 10 msg/min at steady state
- [ ] Tool execution: web.search < 3s p95
- [ ] Vector search: 10K vectors < 100ms
- [ ] Message persistence: < 10ms
- [ ] Session load (1000 msgs): < 200ms
- [ ] API response: < 50ms p95 (no model call)
- [ ] Binary size: < 150MB uncompressed

### 28.2 Quality Targets

- [ ] Test coverage: ≥ 80% overall
- [ ] Integration tests: all critical paths
- [ ] Benchmarks: documented baseline
- [ ] Error states: all have user-friendly messages
- [ ] Loading states: all async ops covered

---

## 29. Multi-Model Group Chat

### 29.1 Concept

- [ ] Multiple AI models converse in a shared chat room
- [ ] Models have distinct personalities, names, and system prompts
- [ ] User can participate in the conversation or just observe
- [ ] Models can call tools, use memory, and access user context
- [ ] Experimental feature, clearly marked as "Experimental" in UI

### 29.2 Room Configuration

- [ ] Room: name, description, max_participants, topic
- [ ] Participants: [{model_id, name, personality_prompt, color, avatar}]
- [ ] Personality prompts define how each model behaves:
  - [ ] "You are a sarcastic AI who loves roasting bad code"
  - [ ] "You are a philosophical AI who answers everything with questions"
  - [ ] "You are a enthusiastic AI who is overly excited about everything"
- [ ] Built-in personality presets: snarky, philosophical, excited, grumpy, formal, poetic, pirate
- [ ] Room topic sets initial context: "Discuss the meaning of life" or "Review this pull request"
- [ ] Max participants: 8 (configurable)
- [ ] Room persistence: saved as sessions with special type="group_chat"

### 29.3 Conversation Flow

- [ ] Round-robin by default: each model responds in turn
- [ ] Configurable turn order: round_robin, random, free_for_all, user_directed
- [ ] free_for_all: any model can respond to any message
- [ ] user_directed: user @mentions specific model to respond
- [ ] Turn timer: model has N seconds to respond or is skipped
- [ ] Default turn timer: 30s
- [ ] Response length limit: max 500 tokens per model per turn
- [ ] Full conversation history visible to all participants
- [ ] Each model sees: room topic, full history, its personality prompt
- [ ] Models see their own name/tag in the system prompt
- [ ] Models do NOT see other models' personality prompts (unless configured)
- [ ] Models can reference each other: "@SnarkyBot what do you think?"
- [ ] @mentions parsed and passed as context to the mentioned model

### 29.4 UI

- [ ] Chat area with color-coded messages per model
- [ ] Model avatar/icon next to each message
- [ ] Online indicator per model
- [ ] Typing indicator when model is generating
- [ ] "Thinking..." state with elapsed time
- [ ] Skip button: force-skip current model's turn
- [ ] Pause/Resume button
- [ ] Speed control: 1x, 2x, 4x (skip typing delay)
- [ ] Participant list sidebar with personalities
- [ ] "Add model" button → model selector dialog
- [ ] "Remove model" from context menu
- [ ] User messages appear in distinct style (different from models)
- [ ] User can @mention a specific model to direct the question

### 29.5 Model Integration

- [ ] Each participant uses a configured model provider+model
- [ ] Different models can use different providers
- [ ] Each model session is independent (separate context windows)
- [ ] Context window per model: built from shared history + their personality
- [ ] Token counting per participant in status bar
- [ ] Cost tracking per participant
- [ ] Rate limits: per-model, not per-room
- [ ] All standard tools available to each model
- [ ] memory.query tool works for all models (shared memory)
- [ ] Refusal detection still applies per model (safety)
- [ ] If one model refuses → other models can still respond
- [ ] Fallback chains per model configuration

### 29.6 Edge Cases

- [ ] Model generates infinite loop (talking to itself) → detect, interrupt
- [ ] Model outputs very long message → truncate at 500 tokens, "..." append
- [ ] Model outputs harmful content → per-model safety still applies
- [ ] Model goes offline (provider down) → remove from room, notify
- [ ] Room with 1 model → degrades to regular chat (but with personality)
- [ ] All models refuse → room ends, error state shown
- [ ] User adds 8 models → UI must handle 8 simultaneous streaming responses
- [ ] Conversation gets very long (1000+ messages) → summarization per model's context
- [ ] Model responds to wrong message (race condition) → sequential turn enforcement
- [ ] User deletes a message → all models regenerate responses

### 29.7 Tests

- [ ] Test: 2 models converse in round-robin
- [ ] Test: @mention directs response to specific model
- [ ] Test: turn timer skips unresponsive model
- [ ] Test: refusal in one model does not block others
- [ ] Test: personality prompt changes model behavior
- [ ] Test: room persistence survives restart
- [ ] Test: free_for_all mode allows interleaved responses
- [ ] Test: user message interrupts model conversation

---

## 30. What We Are NOT Building

- [ ] No macOS support (Linux only)
- [ ] No Windows support
- [ ] No web UI (desktop native + API)
- [ ] No mobile app
- [ ] No SaaS/cloud hosted version
- [ ] No model training or fine-tuning
- [ ] No Kubernetes orchestration
- [ ] No external database (SQLite only)
- [ ] No user accounts/multi-user (single user)
- [ ] No collaborative features
- [ ] No social features (sharing, following)
- [ ] No audio/video processing
- [ ] No real-time voice chat
- [ ] No browser extension
- [ ] No content moderation pipeline
- [ ] No A/B testing framework
- [ ] No Prometheus/Grafana integration
- [ ] No CI/CD integration
