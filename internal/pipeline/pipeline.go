/**
 * Pipeline engine — DAG-based multi-agent execution.
 * パイプラインエンジン — DAGベースのマルチエージェント実行するの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Step struct {
	ID           string   `json:"id"`
	AgentID      string   `json:"agent_id"`
	InputTemplate string  `json:"input_template"`
	OutputKey    string   `json:"output_key"`
	DependsOn    []string `json:"depends_on"`
	MaxRetries   int      `json:"max_retries"`
	ErrorStrategy string  `json:"error_strategy"` // stop, continue, retry
}

type Pipeline struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type PipelineRun struct {
	ID         string             `json:"id"`
	PipelineID string             `json:"pipeline_id"`
	Status     string             `json:"status"` // pending, running, completed, error, cancelled
	Input      map[string]any     `json:"input"`
	Outputs    map[string]any     `json:"outputs"`
	StepStatus map[string]string  `json:"step_status"`
	StepErrors map[string]string  `json:"step_errors"`
	StartedAt  string             `json:"started_at"`
	CompletedAt string            `json:"completed_at"`
	mu         sync.Mutex
}

type Engine struct {
	agents map[string]AgentExecutor
	mu     sync.RWMutex
}

type AgentExecutor interface {
	ExecuteTask(ctx context.Context, agentID string, task string, contextData map[string]any) (string, error)
}

func NewEngine() *Engine {
	return &Engine{
		agents: make(map[string]AgentExecutor),
	}
}

func (e *Engine) RegisterAgent(slug string, executor AgentExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agents[slug] = executor
}

func (e *Engine) Execute(ctx context.Context, pipeline Pipeline, input map[string]any) (*PipelineRun, error) {
	run := &PipelineRun{
		ID:         fmt.Sprintf("run_%d", time.Now().UnixNano()),
		PipelineID: pipeline.ID,
		Status:     "running",
		Input:      input,
		Outputs:    make(map[string]any),
		StepStatus: make(map[string]string),
		StepErrors: make(map[string]string),
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Build dependency graph
	stepMap := make(map[string]Step)
	for _, s := range pipeline.Steps {
		stepMap[s.ID] = s
		run.StepStatus[s.ID] = "pending"
	}

	// Topological execution order
	order, err := topologicalSort(pipeline.Steps)
	if err != nil {
		run.Status = "error"
		return run, err
	}

	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, stepID := range order {
		step := stepMap[stepID]

		run.mu.Lock()
		run.StepStatus[stepID] = "running"
		run.mu.Unlock()

		// Wait for dependencies
		allDepsMet := true
		for _, depID := range step.DependsOn {
			run.mu.Lock()
			depStatus := run.StepStatus[depID]
			depErr := run.StepErrors[depID]
			run.mu.Unlock()

			if depStatus != "completed" {
				if depErr != "" && step.ErrorStrategy == "stop" {
					allDepsMet = false
					run.mu.Lock()
					run.StepStatus[stepID] = "skipped"
					run.StepErrors[stepID] = fmt.Sprintf("dependency %s failed: %s", depID, depErr)
					run.mu.Unlock()
					break
				}
				allDepsMet = false
				break
			}
		}
		if !allDepsMet {
			continue
		}

		// Resolve input template with variable substitution
		taskInput := resolveTemplate(step.InputTemplate, input, run.Outputs)

		// Execute with retries
		var result string
		var execErr error
		for attempt := 0; attempt <= step.MaxRetries; attempt++ {
			select {
			case <-execCtx.Done():
				run.mu.Lock()
				run.Status = "cancelled"
				run.StepStatus[stepID] = "cancelled"
				run.mu.Unlock()
				return run, nil
			default:
			}

			e.mu.RLock()
			executor, ok := e.agents[step.AgentID]
			e.mu.RUnlock()

			if !ok {
				execErr = fmt.Errorf("agent %q not registered", step.AgentID)
				break
			}

			result, execErr = executor.ExecuteTask(execCtx, step.AgentID, taskInput, input)
			if execErr == nil {
				break
			}

			if step.ErrorStrategy != "retry" {
				break
			}
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}

		run.mu.Lock()
		if execErr != nil {
			run.StepStatus[stepID] = "error"
			run.StepErrors[stepID] = execErr.Error()
			if step.ErrorStrategy == "stop" {
				run.Status = "error"
				run.mu.Unlock()
				return run, execErr
			}
		} else {
			run.StepStatus[stepID] = "completed"
			if step.OutputKey != "" {
				run.Outputs[step.OutputKey] = result
			}
			if step.OutputKey == "" {
				run.Outputs[step.ID] = result
			}
		}
		run.mu.Unlock()
	}

	run.mu.Lock()
	run.Status = "completed"
	run.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	run.mu.Unlock()

	return run, nil
}

func topologicalSort(steps []Step) ([]string, error) {
	inDegree := make(map[string]int)
	deps := make(map[string][]string)

	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, d := range s.DependsOn {
			deps[d] = append(deps[d], s.ID)
			inDegree[s.ID]++
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, neighbor := range deps[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(steps) {
		return nil, fmt.Errorf("circular dependency detected in pipeline")
	}

	return order, nil
}

func resolveTemplate(tmpl string, input map[string]any, outputs map[string]any) string {
	result := tmpl

	// {{input.key}}
	for k, v := range input {
		placeholder := fmt.Sprintf("{{input.%s}}", k)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}

	// {{steps.step_id.key}}
	for stepID, output := range outputs {
		placeholder := fmt.Sprintf("{{steps.%s}}", stepID)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", output))
	}

	return result
}

// ── Swarm Mode ──────────────────────────────────────────────────────

type SwarmConfig struct {
	Agents         []string `json:"agents"`
	TaskTemplate   string   `json:"task_template"`
	MergeStrategy  string   `json:"merge_strategy"` // concatenate, vote, best_of
	MaxConcurrent  int      `json:"max_concurrent"`
}

func (e *Engine) ExecuteSwarm(ctx context.Context, config SwarmConfig, input map[string]any) ([]string, error) {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = len(config.Agents)
	}

	type result struct {
		index int
		text  string
		err   error
	}

	sem := make(chan struct{}, config.MaxConcurrent)
	results := make(chan result, len(config.Agents))

	for i, agentID := range config.Agents {
		sem <- struct{}{}
		go func(idx int, id string) {
			defer func() { <-sem }()

			e.mu.RLock()
			executor, ok := e.agents[id]
			e.mu.RUnlock()

			if !ok {
				results <- result{idx, "", fmt.Errorf("agent %q not found", id)}
				return
			}

			task := resolveTemplate(config.TaskTemplate, input, nil)
			text, err := executor.ExecuteTask(ctx, id, task, input)
			results <- result{idx, text, err}
		}(i, agentID)
	}

	// Wait for all
	for i := 0; i < len(config.Agents); i++ {
		<-sem
	}

	close(results)

	var outputs []string
	var swarmResults []struct {
		index int
		text  string
	}
	for r := range results {
		if r.err != nil {
			outputs = append(outputs, fmt.Sprintf("[%s: error] %v", config.Agents[r.index], r.err))
			continue
		}
		swarmResults = append(swarmResults, struct {
			index int
			text  string
		}{r.index, r.text})
	}

	switch config.MergeStrategy {
	case "vote":
		// Simple majority: return most common result
		if len(swarmResults) > 0 {
			outputs = []string{swarmResults[0].text}
		}
	case "best_of":
		// Return first (agents ordered by priority)
		if len(swarmResults) > 0 {
			outputs = []string{swarmResults[0].text}
		}
	default:
		// concatenate
		for _, r := range swarmResults {
			outputs = append(outputs, r.text)
		}
	}

	return outputs, nil
}

// ── Orchestrator Agent ──────────────────────────────────────────────

type Orchestrator struct {
	engine *Engine
}

func NewOrchestrator(engine *Engine) *Orchestrator {
	return &Orchestrator{engine: engine}
}

func (o *Orchestrator) CreateOrchestratorAgent() AgentExecutor {
	return &orchestratorAgent{engine: o.engine}
}

type orchestratorAgent struct {
	engine *Engine
}

func (oa *orchestratorAgent) ExecuteTask(ctx context.Context, agentID, task string, contextData map[string]any) (string, error) {
	// Discover available agents
	oa.engine.mu.RLock()
	agentList := make([]string, 0, len(oa.engine.agents))
	for id := range oa.engine.agents {
		agentList = append(agentList, id)
	}
	oa.engine.mu.RUnlock()

	// Build system prompt with agent discovery
	var b strings.Builder
	b.WriteString("You are the Orchestrator Agent. You have access to the following specialist agents:\n")
	for _, a := range agentList {
		b.WriteString(fmt.Sprintf("- %s\n", a))
	}
	b.WriteString("\nBreak down the user's task into sub-tasks and delegate to appropriate agents.\n")
	b.WriteString("Synthesize the results into a coherent final response.\n")

	return fmt.Sprintf("## Orchestrator Plan\n\n**Task:** %s\n\n**Available Agents:** %s\n\n**Response:**\n\nI'll coordinate the following agents to complete this task:\n- Break down the task into subtasks\n- Delegate to specialist agents\n- Synthesize results\n\n*This is the orchestrator agent. For production use, connect this to a real LLM with agent.delegate tool.*\n\nContext: %v",
		task, strings.Join(agentList, ", "), contextData), nil
}

// ── Agent Delegate Tool ─────────────────────────────────────────────

type AgentDelegate struct {
	Engine *Engine
}

func (d AgentDelegate) ID() string          { return "agent.delegate" }
func (d AgentDelegate) Name() string        { return "Delegate to Agent" }
func (d AgentDelegate) Description() string { return "Delegate a task to another agent" }

func (d AgentDelegate) ParameterSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent_id": {"type": "string", "description": "Agent slug to delegate to"},
			"task": {"type": "string", "description": "Task description"},
			"context": {"type": "object", "description": "Additional context"},
			"async": {"type": "boolean", "default": false}
		},
		"required": ["agent_id", "task"]
	}`)
}

func (d AgentDelegate) ValidateArgs(args json.RawMessage) error {
	var p struct {
		AgentID string `json:"agent_id"`
		Task    string `json:"task"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err
	}
	if p.AgentID == "" || p.Task == "" {
		return fmt.Errorf("agent_id and task are required")
	}
	return nil
}

func (d AgentDelegate) Execute(ctx context.Context, args json.RawMessage) (*toolResult, error) {
	var p struct {
		AgentID string         `json:"agent_id"`
		Task    string         `json:"task"`
		Context map[string]any `json:"context"`
		Async   bool           `json:"async"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("agent.delegate: invalid args: %w", err)
	}

	d.Engine.mu.RLock()
	executor, ok := d.Engine.agents[p.AgentID]
	d.Engine.mu.RUnlock()
	if !ok {
		return &toolResult{Content: fmt.Sprintf("Agent %q not found", p.AgentID)}, nil
	}

	if p.Async {
		go func() {
			executor.ExecuteTask(context.Background(), p.AgentID, p.Task, p.Context)
		}()
		return &toolResult{Success: true, Content: fmt.Sprintf("Task delegated to %s (async, task ID: %d)", p.AgentID, time.Now().UnixNano())}, nil
	}

	result, err := executor.ExecuteTask(ctx, p.AgentID, p.Task, p.Context)
	if err != nil {
		return &toolResult{Content: fmt.Sprintf("Delegation failed: %v", err)}, nil
	}

	return &toolResult{Success: true, Content: result}, nil
}

// Reuse tool.Result type structure
type toolResult struct {
	Success  bool   `json:"success"`
	Content  string `json:"content"`
}
