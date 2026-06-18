package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/KleaSCM/nala/internal/agent"
	"github.com/KleaSCM/nala/internal/db"
	"github.com/KleaSCM/nala/internal/memory"
	"github.com/KleaSCM/nala/internal/model"
	"github.com/KleaSCM/nala/internal/pipeline"
	"github.com/KleaSCM/nala/internal/scheduler"
	"github.com/KleaSCM/nala/internal/tool"
)

func (a *App) CreateAgent(name, systemPrompt, personality string, params agentParams) (*agent.Agent, error) {
	ag := &agent.Agent{
		Name:         name,
		SystemPrompt: systemPrompt,
		Personality:  personality,
	}
	if params.MaxTokens > 0 {
		ag.MaxTokens = params.MaxTokens
	}
	if params.Temperature > 0 {
		ag.Temperature = params.Temperature
	}
	if params.TopP > 0 {
		ag.TopP = params.TopP
	}
	if params.TimeoutMS > 0 {
		ag.TimeoutMS = params.TimeoutMS
	}
	if params.MaxRetries > 0 {
		ag.MaxRetries = params.MaxRetries
	}
	if len(params.ToolIDs) > 0 {
		ag.ToolBindings = fmt.Sprintf(`[%s]`, joinQuoted(params.ToolIDs, ","))
	}
	if err := a.engine.AgentManager.Create(ag); err != nil {
		return nil, err
	}
	return ag, nil
}

func (a *App) GetAgent(id string) (*agent.Agent, error) {
	return a.engine.AgentManager.Get(id)
}

type listAgentsFilter struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Name   string `json:"name"`
}

func (a *App) ListAgents(filter listAgentsFilter) ([]*agent.Agent, error) {
	return a.engine.AgentManager.List(agent.AgentFilter{
		Limit:  filter.Limit,
		Offset: filter.Offset,
		Name:   filter.Name,
	})
}

func (a *App) UpdateAgent(ag *agent.Agent) error {
	return a.engine.AgentManager.Update(ag)
}

func (a *App) DeleteAgent(id string) error {
	return a.engine.AgentManager.Delete(id)
}

// ── Session ──────────────────────────────────────────────────────────

func (a *App) CreateSession(agentID string) (*agent.Session, error) {
	s := &agent.Session{
		AgentID: agentID,
	}
	if err := a.engine.SessionManager.Create(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (a *App) GetSession(id string) (*agent.Session, error) {
	return a.engine.SessionManager.Get(id)
}

type listSessionsFilter struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (a *App) ListSessions(filter listSessionsFilter) ([]*agent.Session, error) {
	return a.engine.SessionManager.List(agent.SessionFilter{
		AgentID: filter.AgentID,
		Status:  filter.Status,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
}

func (a *App) DeleteSession(id string) error {
	return a.engine.SessionManager.Delete(id)
}

func (a *App) PauseSession(id string) error {
	return a.engine.SessionManager.Pause(id)
}

func (a *App) ResumeSession(id string) error {
	return a.engine.SessionManager.Resume(id)
}

// ── Chat ─────────────────────────────────────────────────────────────

func (a *App) SendMessage(sessionID, message string) (*agent.TurnResult, error) {
	return a.engine.ConversationLoop.ProcessMessage(context.Background(), sessionID, message)
}

// ── Model / Provider ──────────────────────────────────────────────────

type providerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *App) ListProviders() []providerInfo {
	providers := a.engine.ModelRegistry.List()
	infos := make([]providerInfo, 0, len(providers))
	for _, p := range providers {
		infos = append(infos, providerInfo{ID: p.ID(), Name: p.Name()})
	}
	return infos
}

func (a *App) ListModels(providerID string) ([]model.ModelInfo, error) {
	p, err := a.engine.ModelRegistry.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.ListModels(context.Background())
}

// ── Settings ──────────────────────────────────────────────────────────

func (a *App) GetAppSetting(key string) (string, error) {
	return db.GetAppState(a.engine.DB, key)
}

func (a *App) SetAppSetting(key, value string) error {
	return db.SetAppState(a.engine.DB, key, value)
}

// ── Helpers ───────────────────────────────────────────────────────────

type agentParams struct {
	MaxTokens   int      `json:"max_tokens"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	TimeoutMS   int      `json:"timeout_ms"`
	MaxRetries  int      `json:"max_retries"`
	ToolIDs     []string `json:"tool_ids"`
}

func joinQuoted(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := `"` + items[0] + `"`
	for i := 1; i < len(items); i++ {
		result += sep + `"` + items[i] + `"`
	}
	return result
}

// ── Memory / Knowledge Base ─────────────────────────────────────────────

type memoryKbInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	EmbeddingModel string `json:"embedding_model"`
	ChunkStrategy  string `json:"chunk_strategy"`
	ChunkSize      int    `json:"chunk_size"`
	ChunkOverlap   int    `json:"chunk_overlap"`
	DocumentCount  int    `json:"document_count"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func (a *App) ListKnowledgeBases() ([]memoryKbInfo, error) {
	if a.engine.MemoryManager == nil {
		return nil, fmt.Errorf("memory manager not available")
	}
	kbs, err := a.engine.MemoryManager.ListKBs(context.Background())
	if err != nil {
		return nil, err
	}
	infos := make([]memoryKbInfo, 0, len(kbs))
	for _, kb := range kbs {
		infos = append(infos, memoryKbInfo{
			ID:             kb.ID,
			Name:           kb.Name,
			Description:    kb.Description,
			EmbeddingModel: kb.EmbeddingModel,
			ChunkStrategy:  kb.ChunkStrategy,
			ChunkSize:      kb.ChunkSize,
			ChunkOverlap:   kb.ChunkOverlap,
			DocumentCount:  kb.DocumentCount,
			CreatedAt:      kb.CreatedAt,
			UpdatedAt:      kb.UpdatedAt,
		})
	}
	return infos, nil
}

func (a *App) CreateKnowledgeBase(name, description, embeddingModel, chunkStrategy string, chunkSize, chunkOverlap int) (*memoryKbInfo, error) {
	if a.engine.MemoryManager == nil {
		return nil, fmt.Errorf("memory manager not available")
	}
	kb, err := a.engine.MemoryManager.CreateKB(context.Background(), name, embeddingModel, chunkStrategy, chunkSize, chunkOverlap)
	if err != nil {
		return nil, err
	}
	return &memoryKbInfo{
		ID:             kb.ID,
		Name:           kb.Name,
		Description:    kb.Description,
		EmbeddingModel: kb.EmbeddingModel,
		ChunkStrategy:  kb.ChunkStrategy,
		ChunkSize:      kb.ChunkSize,
		ChunkOverlap:   kb.ChunkOverlap,
		DocumentCount:  kb.DocumentCount,
		CreatedAt:      kb.CreatedAt,
		UpdatedAt:      kb.UpdatedAt,
	}, nil
}

func (a *App) DeleteKnowledgeBase(id string) error {
	if a.engine.MemoryManager == nil {
		return fmt.Errorf("memory manager not available")
	}
	return a.engine.MemoryManager.DeleteKB(context.Background(), id)
}

type memoryDocumentInfo struct {
	ID        string `json:"id"`
	KBID      string `json:"kb_id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	CreatedAt string `json:"created_at"`
}

func (a *App) IngestDocument(kbID, filePath string) (*memoryDocumentInfo, error) {
	if a.engine.MemoryManager == nil {
		return nil, fmt.Errorf("memory manager not available")
	}
	doc, err := a.engine.MemoryManager.IngestDocument(context.Background(), filePath, kbID)
	if err != nil {
		return nil, err
	}
	return &memoryDocumentInfo{
		ID:        doc.ID,
		KBID:      doc.KBID,
		Filename:  doc.Filename,
		MimeType:  doc.MimeType,
		CreatedAt: doc.CreatedAt,
	}, nil
}

type memoryQueryResult struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
	Hops    int      `json:"hops"`
}

func (a *App) QueryMemory(query string, maxHops int) (*memoryQueryResult, error) {
	if a.engine.MemoryManager == nil {
		return nil, fmt.Errorf("memory manager not available")
	}
	result, err := a.engine.MemoryManager.MultiHopQuery(context.Background(), query, maxHops)
	if err != nil {
		return nil, err
	}
	sources := make([]string, len(result.Sources))
	for i, s := range result.Sources {
		sources[i] = s.Content
	}
	return &memoryQueryResult{
		Answer:  result.Answer,
		Sources: sources,
		Hops:    result.Hops,
	}, nil
}

type userMemoryInfo struct {
	Fact       string  `json:"fact"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
	CreatedAt  string  `json:"created_at"`
}

func (a *App) ListUserMemories() ([]userMemoryInfo, error) {
	if a.engine.MemoryManager == nil {
		return nil, fmt.Errorf("memory manager not available")
	}
	mems := a.engine.MemoryManager.GetAllUserMemories()
	infos := make([]userMemoryInfo, 0, len(mems))
	for _, m := range mems {
		infos = append(infos, userMemoryInfo{
			Fact:       m.Fact,
			Category:   m.Category,
			Confidence: m.Confidence,
			Source:     m.Source,
			CreatedAt:  m.CreatedAt.Format(time.RFC3339),
		})
	}
	return infos, nil
}

func (a *App) StoreUserMemory(fact, category string, importance float64) error {
	if a.engine.MemoryManager == nil {
		return fmt.Errorf("memory manager not available")
	}
	return a.engine.MemoryManager.StoreUserMemory(context.Background(), fact, category, importance)
}

// ── Pipeline ────────────────────────────────────────────────────────────

type pipelineInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       int    `json:"steps"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (a *App) ExecutePipeline(pipelineJSON string, inputJSON string) (string, error) {
	if a.engine.PipelineEngine == nil {
		return "", fmt.Errorf("pipeline engine not available")
	}
	var p pipeline.Pipeline
	if err := json.Unmarshal([]byte(pipelineJSON), &p); err != nil {
		return "", fmt.Errorf("invalid pipeline JSON: %w", err)
	}
	var input map[string]any
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			return "", fmt.Errorf("invalid input JSON: %w", err)
		}
	}
	run, err := a.engine.PipelineEngine.Execute(context.Background(), p, input)
	if err != nil {
		return "", err
	}
	outJSON, _ := json.Marshal(run)
	return string(outJSON), nil
}

func (a *App) ListPipelineAgents() (string, error) {
	// Returns registered agent slugs as JSON array
	infos := []map[string]string{
		{"slug": "default", "name": "Default Agent"},
		{"slug": "orchestrator", "name": "Orchestrator"},
	}
	out, _ := json.Marshal(infos)
	return string(out), nil
}

// ── Scheduler ───────────────────────────────────────────────────────────

type scheduledTaskInfo struct {
	ID             string `json:"id"`
	AgentID        string `json:"agent_id"`
	CronExpression string `json:"cron_expression"`
	InputTemplate  string `json:"input_template"`
	Enabled        bool   `json:"enabled"`
	LastRunAt      string `json:"last_run_at"`
	LastRunStatus  string `json:"last_run_status"`
	NextRunAt      string `json:"next_run_at"`
	MaxRuns        int    `json:"max_runs"`
	RunCount       int    `json:"run_count"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func taskToInfo(task scheduler.ScheduledTask) scheduledTaskInfo {
	info := scheduledTaskInfo{
		ID:             task.ID,
		AgentID:        task.AgentID,
		CronExpression: task.CronExpression,
		InputTemplate:  task.InputTemplate,
		Enabled:        task.Enabled,
		MaxRuns:        task.MaxRuns,
		RunCount:       task.RunCount,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
	if task.LastRunAt != nil {
		info.LastRunAt = *task.LastRunAt
	}
	if task.LastRunStatus != nil {
		info.LastRunStatus = *task.LastRunStatus
	}
	if task.NextRunAt != nil {
		info.NextRunAt = *task.NextRunAt
	}
	return info
}

func (a *App) ListScheduledTasks() ([]scheduledTaskInfo, error) {
	if a.engine.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not available")
	}
	tasks := a.engine.Scheduler.ListTasks()
	infos := make([]scheduledTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		infos = append(infos, taskToInfo(t))
	}
	return infos, nil
}

func (a *App) ScheduleTask(agentID, cronExpression, inputTemplate string, maxRuns int) (*scheduledTaskInfo, error) {
	if a.engine.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not available")
	}
	task := scheduler.ScheduledTask{
		AgentID:        agentID,
		CronExpression: cronExpression,
		InputTemplate:  inputTemplate,
		Enabled:        true,
		MaxRuns:        maxRuns,
	}
	if err := a.engine.Scheduler.Schedule(task); err != nil {
		return nil, err
	}
	result := taskToInfo(task)
	return &result, nil
}

func (a *App) UnscheduleTask(id string) error {
	if a.engine.Scheduler == nil {
		return fmt.Errorf("scheduler not available")
	}
	a.engine.Scheduler.Unschedule(id)
	return nil
}

func (a *App) ToggleScheduledTask(id string, enabled bool) error {
	if a.engine.Scheduler == nil {
		return fmt.Errorf("scheduler not available")
	}
	return a.engine.Scheduler.Toggle(id, enabled)
}

// ── Notes ───────────────────────────────────────────────────────────────

type noteInfo struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Tags  string `json:"tags"`
	CreatedAt string `json:"created_at"`
}

func (a *App) CreateNote(title, content, tags string) (*noteInfo, error) {
	notesDir := filepath.Join(a.engine.Config.Core.DataDir, "notes")
	create := tool.NotesCreate{NotesDir: notesDir}
	result, err := create.Execute(context.Background(), tool.ToolInput{
		Args: map[string]any{
			"title":   title,
			"content": content,
			"tags":    tags,
		},
	})
	if err != nil {
		return nil, err
	}
	info := &noteInfo{
		Title: title,
		Path:  result.Result,
		Tags:  tags,
	}
	if t, ok := result.Data["created_at"]; ok {
		info.CreatedAt = fmt.Sprintf("%v", t)
	}
	return info, nil
}

func (a *App) ListNotes() ([]noteInfo, error) {
	notesDir := filepath.Join(a.engine.Config.Core.DataDir, "notes")
	list := tool.NotesList{NotesDir: notesDir}
	result, err := list.Execute(context.Background(), tool.ToolInput{Args: map[string]any{}})
	if err != nil {
		return nil, err
	}
	notesRaw, ok := result.Data["notes"]
	if !ok {
		return nil, nil
	}
	notesJSON, err := json.Marshal(notesRaw)
	if err != nil {
		return nil, err
	}
	var notes []noteInfo
	if err := json.Unmarshal(notesJSON, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// ── System ──────────────────────────────────────────────────────────────

type systemStats struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
	Network string `json:"network"`
}

func (a *App) GetSystemStats() (*systemStats, error) {
	monitor := tool.SystemMonitor{}
	result, err := monitor.Execute(context.Background(), tool.ToolInput{Args: map[string]any{}})
	if err != nil {
		return nil, err
	}
	stats := &systemStats{}
	if v, ok := result.Data["cpu"]; ok {
		stats.CPU = fmt.Sprintf("%v", v)
	}
	if v, ok := result.Data["memory"]; ok {
		stats.Memory = fmt.Sprintf("%v", v)
	}
	if v, ok := result.Data["disk"]; ok {
		stats.Disk = fmt.Sprintf("%v", v)
	}
	if v, ok := result.Data["network"]; ok {
		stats.Network = fmt.Sprintf("%v", v)
	}
	return stats, nil
}

type processInfo struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Status  string `json:"status"`
	Command string `json:"command"`
}

func (a *App) GetSystemProcesses(sortBy string, limit int) ([]processInfo, error) {
	procs := tool.SystemProcesses{}
	args := map[string]any{}
	if sortBy != "" {
		args["sort_by"] = sortBy
	}
	if limit > 0 {
		args["limit"] = limit
	}
	result, err := procs.Execute(context.Background(), tool.ToolInput{Args: args})
	if err != nil {
		return nil, err
	}
	procsRaw, ok := result.Data["processes"]
	if !ok {
		return nil, nil
	}
	procsJSON, err := json.Marshal(procsRaw)
	if err != nil {
		return nil, err
	}
	var procList []processInfo
	if err := json.Unmarshal(procsJSON, &procList); err != nil {
		return nil, err
	}
	return procList, nil
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"`
}

func (a *App) GetSystemLogs(lines int) ([]logEntry, error) {
	logs := tool.SystemLogs{LogDir: a.engine.Config.Core.LogFile}
	result, err := logs.Execute(context.Background(), tool.ToolInput{Args: map[string]any{"lines": lines}})
	if err != nil {
		return nil, err
	}
	logsRaw, ok := result.Data["logs"]
	if !ok {
		return nil, nil
	}
	logsJSON, err := json.Marshal(logsRaw)
	if err != nil {
		return nil, err
	}
	var entries []logEntry
	if err := json.Unmarshal(logsJSON, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ── Tool Execution (for frontend tool playground) ───────────────────────

type toolExecuteRequest struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type toolExecuteResponse struct {
	Result string            `json:"result"`
	Data   map[string]any    `json:"data"`
	Error  string            `json:"error,omitempty"`
}

func (a *App) ExecuteTool(name string, argsJSON string) (string, error) {
	toolInput := tool.ToolInput{Args: make(map[string]any)}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &toolInput.Args); err != nil {
			return "", fmt.Errorf("invalid args JSON: %w", err)
		}
	}
	result, err := a.engine.ToolRegistry.Execute(context.Background(), name, toolInput)
	if err != nil {
		resp := toolExecuteResponse{Error: err.Error()}
		out, _ := json.Marshal(resp)
		return string(out), nil
	}
	resp := toolExecuteResponse{
		Result: result.Result,
		Data:   result.Data,
	}
	out, _ := json.Marshal(resp)
	return string(out), nil
}

func (a *App) ListTools() (string, error) {
	tools := a.engine.ToolRegistry.List()
	type toolDesc struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	descs := make([]toolDesc, 0, len(tools))
	for _, t := range tools {
		descs = append(descs, toolDesc{Name: t.Name, Description: t.Description})
	}
	out, _ := json.Marshal(descs)
	return string(out), nil
}

// ── Agent Info (extended) ───────────────────────────────────────────────

func (a *App) GetAgentSessionCount(agentID string) (int, error) {
	sessions, err := a.engine.SessionManager.List(agent.SessionFilter{AgentID: agentID})
	if err != nil {
		return 0, err
	}
	return len(sessions), nil
}

func (a *App) GetSessionMessages(sessionID string) (string, error) {
	messages, err := a.engine.SessionManager.GetMessages(sessionID)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(messages)
	return string(out), nil
}
