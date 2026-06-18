/**
 * Cron-based agent task scheduler.
 * cronベースのエージェントタスクスケジューラーね。
 *
 * Uses robfig/cron/v3 for 5-field expressions with timezone support.
 * robfig/cron/v3を使って、タイムゾーン対応の5フィールド式で動いてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, agentID string, task string, contextData map[string]any) (string, error)
}

type Scheduler struct {
	DB        *sql.DB
	Executor  TaskExecutor
	cron      *cron.Cron
	entries   map[string]cron.EntryID
	mu        sync.Mutex
	jobs      map[string]ScheduledTask
}

func New(db *sql.DB, executor TaskExecutor) *Scheduler {
	c := cron.New(cron.WithSeconds())
	c.Start()

	return &Scheduler{
		DB:       db,
		Executor: executor,
		cron:     c,
		entries:  make(map[string]cron.EntryID),
		jobs:     make(map[string]ScheduledTask),
	}
}

func (s *Scheduler) LoadFromDB(ctx context.Context) error {
	if s.DB == nil {
		return nil
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, agent_id, cron_expression, input_template, enabled, max_runs, run_count, next_run_at, created_at, updated_at
		 FROM scheduled_tasks`)
	if err != nil {
		return fmt.Errorf("scheduler: load tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var task ScheduledTask
		var enabled int64
		var nextRunAt sql.NullString
		if err := rows.Scan(&task.ID, &task.AgentID, &task.CronExpression, &task.InputTemplate,
			&enabled, &task.MaxRuns, &task.RunCount, &nextRunAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
			continue
		}
		task.Enabled = enabled == 1
		if nextRunAt.Valid {
			task.NextRunAt = &nextRunAt.String
		}

		if task.Enabled {
			if err := s.Schedule(task); err != nil {
				fmt.Fprintf(s.stderr(), "scheduler: failed to schedule %s: %v\n", task.ID, err)
			}
		}
	}

	// Catch-up: check for missed runs
	s.catchUp()

	return nil
}

func (s *Scheduler) Schedule(task ScheduledTask) error {
	// Check max_runs
	if task.MaxRuns > 0 && task.RunCount >= task.MaxRuns {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing schedule if any
	if entryID, ok := s.entries[task.ID]; ok {
		s.cron.Remove(entryID)
	}

	job := &taskJob{
		task:     task,
		sched:    s,
	}

	entryID, err := s.cron.AddJob(task.CronExpression, job)
	if err != nil {
		return fmt.Errorf("scheduler: invalid cron expression %q: %w", task.CronExpression, err)
	}

	s.entries[task.ID] = entryID
	s.jobs[task.ID] = task
	return nil
}

func (s *Scheduler) Unschedule(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
		delete(s.jobs, taskID)
	}
}

func (s *Scheduler) ListTasks() []ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]ScheduledTask, 0, len(s.jobs))
	for _, t := range s.jobs {
		tasks = append(tasks, t)
	}
	return tasks
}

func (s *Scheduler) Toggle(taskID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.jobs[taskID]
	if !ok {
		return fmt.Errorf("scheduler: task %s not found", taskID)
	}

	task.Enabled = enabled
	if enabled {
		s.cron.Remove(s.entries[taskID])
		entryID, err := s.cron.AddFunc(task.CronExpression, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			s.executeTask(ctx, task)
		})
		if err != nil {
			return fmt.Errorf("scheduler: re-add task %s: %w", taskID, err)
		}
		s.entries[taskID] = entryID
	} else {
		if entryID, ok := s.entries[taskID]; ok {
			s.cron.Remove(entryID)
			delete(s.entries, taskID)
		}
	}

	s.jobs[taskID] = task
	return nil
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *Scheduler) catchUp() {
	// Check for tasks that should have run while scheduler was offline
	now := time.Now()
	for _, task := range s.jobs {
		if task.NextRunAt != nil {
			nextTime, err := time.Parse(time.RFC3339, *task.NextRunAt)
			if err == nil && nextTime.Before(now) {
				// Missed run — execute now if within catchup window (1 hour)
				if now.Sub(nextTime) < time.Hour {
					go func(t ScheduledTask) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
						defer cancel()
						s.executeTask(ctx, t)
					}(task)
				}
			}
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task ScheduledTask) {
	if s.Executor == nil {
		return
	}

	_, err := s.Executor.ExecuteTask(ctx, task.AgentID, task.InputTemplate, nil)
	if err != nil {
		fmt.Fprintf(s.stderr(), "scheduler: task %s failed: %v\n", task.ID, err)
	}

	// Update run count and last run time
	s.mu.Lock()
	task.RunCount++
	status := "completed"
	task.LastRunStatus = &status
	now := time.Now().UTC().Format(time.RFC3339)
	task.LastRunAt = &now
	s.jobs[task.ID] = task

	// Check max_runs after increment
	if task.MaxRuns > 0 && task.RunCount >= task.MaxRuns {
		s.cron.Remove(s.entries[task.ID])
		delete(s.entries, task.ID)
	}
	s.mu.Unlock()

	// Persist to DB
	if s.DB != nil {
		s.DB.ExecContext(context.Background(),
			`UPDATE scheduled_tasks SET run_count = ?, last_run_at = ?, last_run_status = ? WHERE id = ?`,
			task.RunCount, now, "completed", task.ID)
	}
}

func (s *Scheduler) stderr() io.Writer {
	return os.Stderr
}

type taskJob struct {
	task  ScheduledTask
	sched *Scheduler
}

func (j *taskJob) Run() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	j.sched.executeTask(ctx, j.task)
}
