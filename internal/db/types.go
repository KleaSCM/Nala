/**
 * Shared database type definitions.
 * データベースの共有タイプ定義ね。
 *
 * Defines AuditLogEntry and shared utility types used across subsystems.
 * AuditLogEntryと各サブシステムで使う共有ユーティリティタイプを定義してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package db

type AuditLogEntry struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Category   string `json:"category"`
	Action     string `json:"action"`
	ActorID    string `json:"actor_id,omitempty"`
	ActorType  string `json:"actor_type,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Details    string `json:"details"`
	DurationMS int    `json:"duration_ms,omitempty"`
}
