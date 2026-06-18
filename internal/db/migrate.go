/**
 * Migration runner for SQLite.
 * SQLiteのマイグレーションランナーね。
 *
 * Applies numbered migrations in order, tracking applied ones in _migrations table.
 * 番号順にマイグレーションを適用して、_migrationsテーブルで管理してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package db

import (
	"database/sql"
	"fmt"
)

type Migration struct {
	ID    int
	Name  string
	Query string
}

func Migrations() []Migration {
	return []Migration{
		{
			ID:   1,
			Name: "create_agents",
			Query: `CREATE TABLE IF NOT EXISTS agents (
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
			CREATE INDEX IF NOT EXISTS idx_agents_slug ON agents(slug);
			CREATE INDEX IF NOT EXISTS idx_agents_created ON agents(created_at);`,
		},
		{
			ID:   2,
			Name: "create_sessions",
			Query: `CREATE TABLE IF NOT EXISTS sessions (
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
			CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_id);
			CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
			CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at);`,
		},
		{
			ID:   3,
			Name: "create_messages",
			Query: `CREATE TABLE IF NOT EXISTS messages (
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
			CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
			CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(session_id, created_at);`,
		},
		{
			ID:   4,
			Name: "create_tool_bindings",
			Query: `CREATE TABLE IF NOT EXISTS tool_bindings (
				id         TEXT PRIMARY KEY,
				agent_id   TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
				tool_id    TEXT NOT NULL,
				config     TEXT DEFAULT '{}',
				enabled    INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				UNIQUE(agent_id, tool_id)
			);
			CREATE INDEX IF NOT EXISTS idx_tool_bindings_agent ON tool_bindings(agent_id);`,
		},
		{
			ID:   5,
			Name: "create_knowledge_bases",
			Query: `CREATE TABLE IF NOT EXISTS knowledge_bases (
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
			);`,
		},
		{
			ID:   6,
			Name: "create_documents",
			Query: `CREATE TABLE IF NOT EXISTS documents (
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
			CREATE INDEX IF NOT EXISTS idx_documents_kb ON documents(kb_id);
			CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(content_hash);`,
		},
		{
			ID:   7,
			Name: "create_provider_configs",
			Query: `CREATE TABLE IF NOT EXISTS provider_configs (
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
			CREATE INDEX IF NOT EXISTS idx_providers_type ON provider_configs(provider);
			CREATE INDEX IF NOT EXISTS idx_providers_priority ON provider_configs(priority);`,
		},
		{
			ID:   8,
			Name: "create_scheduled_tasks",
			Query: `CREATE TABLE IF NOT EXISTS scheduled_tasks (
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
			CREATE INDEX IF NOT EXISTS idx_scheduled_next ON scheduled_tasks(enabled, next_run_at);`,
		},
		{
			ID:   9,
			Name: "create_audit_log",
			Query: `CREATE TABLE IF NOT EXISTS audit_log (
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
			CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
			CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_log(category);
			CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_id);`,
		},
		{
			ID:   10,
			Name: "create_fts5_indexes",
			Query: `CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
				content,
				content=messages,
				content_rowid=rowid
			);
			CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
				content,
				content=documents,
				content_rowid=rowid
			);`,
		},
		{
			ID:   11,
			Name: "create_app_state",
			Query: `CREATE TABLE IF NOT EXISTS app_state (
				key   TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);`,
		},
		{
			ID:   12,
			Name: "create_calendars",
			Query: `CREATE TABLE IF NOT EXISTS calendars (
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
			);`,
		},
		{
			ID:   13,
			Name: "create_events",
			Query: `CREATE TABLE IF NOT EXISTS events (
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
			CREATE INDEX IF NOT EXISTS idx_events_calendar ON events(calendar_id);
			CREATE INDEX IF NOT EXISTS idx_events_start ON events(start_time);
			CREATE INDEX IF NOT EXISTS idx_events_end ON events(end_time);`,
		},
		{
			ID:   14,
			Name: "create_email_accounts",
			Query: `CREATE TABLE IF NOT EXISTS email_accounts (
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
			);`,
		},
		{
			ID:   15,
			Name: "create_emails",
			Query: `CREATE TABLE IF NOT EXISTS emails (
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
			CREATE INDEX IF NOT EXISTS idx_emails_account ON emails(account_id);
			CREATE INDEX IF NOT EXISTS idx_emails_thread ON emails(thread_id);
			CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(account_id, folder);
			CREATE INDEX IF NOT EXISTS idx_emails_received ON emails(received_at);
			CREATE INDEX IF NOT EXISTS idx_emails_message_id ON emails(message_id);`,
		},
		{
			ID:   16,
			Name: "create_reminders",
			Query: `CREATE TABLE IF NOT EXISTS reminders (
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
			CREATE INDEX IF NOT EXISTS idx_reminders_remind ON reminders(remind_at, status);
			CREATE INDEX IF NOT EXISTS idx_reminders_event ON reminders(event_id);`,
		},
		{
			ID:   17,
			Name: "create_skills",
			Query: `CREATE TABLE IF NOT EXISTS skills (
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
			CREATE INDEX IF NOT EXISTS idx_skills_category ON skills(category);
			CREATE INDEX IF NOT EXISTS idx_skills_enabled ON skills(is_enabled);`,
		},
		{
			ID:   18,
			Name: "create_user_data",
			Query: `CREATE TABLE IF NOT EXISTS user_data (
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
			CREATE INDEX IF NOT EXISTS idx_user_data_category ON user_data(category);
			CREATE INDEX IF NOT EXISTS idx_user_data_key ON user_data(category, key);`,
		},
		{
			ID:   19,
			Name: "create_system_hooks",
			Query: `CREATE TABLE IF NOT EXISTS system_hooks (
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
			CREATE INDEX IF NOT EXISTS idx_hooks_trigger ON system_hooks(trigger_type);
			CREATE INDEX IF NOT EXISTS idx_hooks_enabled ON system_hooks(enabled);`,
		},
		{
			ID:   20,
			Name: "create_notes",
			Query: `CREATE TABLE IF NOT EXISTS notes (
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
			CREATE INDEX IF NOT EXISTS idx_notes_folder ON notes(folder);
			CREATE INDEX IF NOT EXISTS idx_notes_pinned ON notes(pinned);`,
		},
		{
			ID:   21,
			Name: "create_tasks",
			Query: `CREATE TABLE IF NOT EXISTS tasks (
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
			CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
			CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_date);
			CREATE INDEX IF NOT EXISTS idx_tasks_linked ON tasks(linked_entity_type, linked_entity_id);`,
		},
	}
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("db: cannot create _migrations table: %w", err)
	}
	for _, m := range Migrations() {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE id = ?", m.ID).Scan(&count); err != nil {
			return fmt.Errorf("db: cannot check migration %d: %w", m.ID, err)
		}
		if count > 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("db: cannot begin transaction for migration %d: %w", m.ID, err)
		}
		if _, err := tx.Exec(m.Query); err != nil {
			tx.Rollback()
			return fmt.Errorf("db: migration %d (%s) failed: %w", m.ID, m.Name, err)
		}
		if _, err := tx.Exec("INSERT INTO _migrations (id, name) VALUES (?, ?)", m.ID, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("db: cannot record migration %d: %w", m.ID, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("db: cannot commit migration %d: %w", m.ID, err)
		}
	}
	return nil
}
