package tool

import (
	"context"
	"encoding/json"
)

type Tool interface {
	ID() string
	Name() string
	Description() string
	ParameterSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (*Result, error)
	ValidateArgs(args json.RawMessage) error
}

type Result struct {
	Success    bool   `json:"success"`
	Content    string `json:"content"`
	MimeType   string `json:"mime_type,omitempty"`
	Data       []byte `json:"data,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Truncated  bool   `json:"truncated,omitempty"`
}
