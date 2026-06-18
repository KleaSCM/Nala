/**
 * Scheduled task types for cron-based agent execution.
 * スケジュールタスクタイプ — cronベースのエージェント実行用ね。
 *
 * Defines ScheduledTask with cron expression and run tracking.
 * cron式と実行追跡機能付きのScheduledTaskを定義してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package scheduler

type ScheduledTask struct {
	ID             string  `json:"id"`
	AgentID        string  `json:"agent_id"`
	CronExpression string  `json:"cron_expression"`
	InputTemplate  string  `json:"input_template,omitempty"`
	Enabled        bool    `json:"enabled"`
	LastRunAt      *string `json:"last_run_at,omitempty"`
	LastRunStatus  *string `json:"last_run_status,omitempty"`
	NextRunAt      *string `json:"next_run_at,omitempty"`
	MaxRuns        int     `json:"max_runs"`
	RunCount       int     `json:"run_count"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}
