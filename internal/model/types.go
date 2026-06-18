package model

type ProviderConfig struct {
	ID              string `json:"id"`
	Provider        string `json:"provider"`
	DisplayName     string `json:"display_name,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	APIKeyEncrypted string `json:"api_key_encrypted,omitempty"`
	APIKeyHint      string `json:"api_key_hint,omitempty"`
	Models          string `json:"models"`
	DefaultModel    string `json:"default_model,omitempty"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
	RateLimitRPM    int    `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM    int    `json:"rate_limit_tpm,omitempty"`
	TimeoutMS       int    `json:"timeout_ms"`
	MaxRetries      int    `json:"max_retries"`
	Metadata        string `json:"metadata"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type ProviderType string

const (
	ProviderOllama   ProviderType = "ollama"
	ProviderOpenAI   ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderCustom   ProviderType = "custom"
)

func (p ProviderType) Valid() bool {
	switch p {
	case ProviderOllama, ProviderOpenAI, ProviderAnthropic, ProviderCustom:
		return true
	}
	return false
}
