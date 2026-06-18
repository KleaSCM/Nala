/**
 * Configuration system for Nala.
 * Nalaの設定システムね。
 *
 * Loads TOML config from ~/.config/nala/config.toml with env var overrides.
 * ~/.config/nala/config.toml からTOML設定を読み込んで、環境変数で上書きできるようになってるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
)

type Config struct {
	Core    CoreConfig    `toml:"core"`
	Server  ServerConfig  `toml:"server"`
	Model   ModelConfig   `toml:"model"`
	Memory  MemoryConfig  `toml:"memory"`
	Tools   ToolsConfig   `toml:"tools"`
	Privacy PrivacyConfig `toml:"privacy"`
	UI      UIConfig      `toml:"ui"`
}

type CoreConfig struct {
	DataDir         string `toml:"data_dir"`
	LogLevel        string `toml:"log_level"`
	LogFile         string `toml:"log_file"`
	LogMaxSize      int    `toml:"log_max_size"`
	LogMaxAge       int    `toml:"log_max_age"`
	MaxSessions     int    `toml:"max_sessions"`
	MaxUploadSizeMb int    `toml:"max_upload_size_mb"`
}

type ServerConfig struct {
	Enabled      bool     `toml:"enabled"`
	Host         string   `toml:"host"`
	Port         int      `toml:"port"`
	RequireAuth  bool     `toml:"require_auth"`
	CorsOrigins  []string `toml:"cors_origins"`
	RateLimitRpm int      `toml:"rate_limit_rpm"`
}

type ModelConfig struct {
	DefaultProvider string `toml:"default_provider"`
	DefaultModel    string `toml:"default_model"`
	MaxTokens       int    `toml:"max_tokens"`
	TimeoutS        int    `toml:"timeout_s"`
	MaxConcurrent   int    `toml:"max_concurrent"`
	OpenAIKey       string `toml:"openai_key"`
}

type MemoryConfig struct {
	VectorBackend            string `toml:"vector_backend"`
	DefaultChunkSize         int    `toml:"default_chunk_size"`
	DefaultChunkOverlap      int    `toml:"default_chunk_overlap"`
	SummarizationIntervalMin int    `toml:"summarization_interval_min"`
	AutoExtractFacts         bool   `toml:"auto_extract_facts"`
}

type ToolsConfig struct {
	SandboxDir         string   `toml:"sandbox_dir"`
	CodeExecTimeoutS   int      `toml:"code_exec_timeout_s"`
	AllowedLanguages   []string `toml:"allowed_languages"`
	ToolsNetworkAccess bool     `toml:"tools_network_access"`
	MaxConcurrentTools int      `toml:"max_concurrent_tools"`
}

type PrivacyConfig struct {
	AirgapMode          bool `toml:"airgap_mode"`
	AuditRetentionDays  int  `toml:"audit_retention_days"`
	SanitizeUploads     bool `toml:"sanitize_uploads"`
	DisableTelemetry    bool `toml:"disable_telemetry"`
	SessionExpiryDays   int  `toml:"session_expiry_days"`
}

type UIConfig struct {
	Theme    string `toml:"theme"`
	FontSize int    `toml:"font_size"`
	Language string `toml:"language"`
}

func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		Core: CoreConfig{
			DataDir:         filepath.Join(homeDir, ".local", "share", "nala"),
			LogLevel:        "info",
			LogFile:         "",
			LogMaxSize:      50,
			LogMaxAge:       30,
			MaxSessions:     10,
			MaxUploadSizeMb: 100,
		},
		Server: ServerConfig{
			Enabled:      true,
			Host:         "127.0.0.1",
			Port:         8472,
			RequireAuth:  false,
			CorsOrigins:  []string{"http://localhost:*", "wails://*"},
			RateLimitRpm: 60,
		},
		Model: ModelConfig{
			DefaultProvider: "ollama",
			DefaultModel:    "llama3.2:3b",
			MaxTokens:       4096,
			TimeoutS:        120,
			MaxConcurrent:   4,
		},
		Memory: MemoryConfig{
			VectorBackend:            "sqlite-vec",
			DefaultChunkSize:         1000,
			DefaultChunkOverlap:      200,
			SummarizationIntervalMin: 60,
			AutoExtractFacts:         true,
		},
		Tools: ToolsConfig{
			SandboxDir:         filepath.Join(homeDir, ".local", "share", "nala", "sandbox"),
			CodeExecTimeoutS:   30,
			AllowedLanguages:   []string{"python", "javascript", "go", "bash"},
			ToolsNetworkAccess: false,
			MaxConcurrentTools: 8,
		},
		Privacy: PrivacyConfig{
			AirgapMode:          false,
			AuditRetentionDays:  90,
			SanitizeUploads:     true,
			DisableTelemetry:    true,
			SessionExpiryDays:   30,
		},
		UI: UIConfig{
			Theme:    "system",
			FontSize: 14,
			Language: "en",
		},
	}
}

func configFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "nala", "config.toml"), nil
}

func ensureConfigDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

func Load() (*Config, error) {
	cfg := DefaultConfig()
	configPath, err := configFilePath()
	if err != nil {
		return nil, err
	}
	if err := ensureConfigDir(configPath); err != nil {
		return nil, fmt.Errorf("cannot create config directory: %w", err)
	}
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		f, createErr := os.Create(configPath)
		if createErr != nil {
			return nil, fmt.Errorf("cannot create config file: %w", createErr)
		}
		if encodeErr := toml.NewEncoder(f).Encode(cfg); encodeErr != nil {
			f.Close()
			return nil, fmt.Errorf("cannot write default config: %w", encodeErr)
		}
		f.Close()
	} else {
		if _, decodeErr := toml.DecodeFile(configPath, &cfg); decodeErr != nil {
			return nil, fmt.Errorf("cannot decode config file: %w", decodeErr)
		}
	}
	applyEnvOverrides(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	setCurrent(cfg)
	return &cfg, nil
}

func Get() Config {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current
}

type ReloadCallback func(old, new Config)
type reloadEntry struct {
	fn       ReloadCallback
	immediate bool
}

var (
	current   Config
	currentMu sync.RWMutex

	reloadMu     sync.Mutex
	reloadCalls  []reloadEntry
	watcher      *fsnotify.Watcher
	watcherDone  chan struct{}
	hotReloadErr error
)

func OnReload(fn ReloadCallback, immediate bool) {
	reloadMu.Lock()
	defer reloadMu.Unlock()
	reloadCalls = append(reloadCalls, reloadEntry{fn: fn, immediate: immediate})
}

func notifyReload(old, new Config) {
	reloadMu.Lock()
	calls := make([]reloadEntry, len(reloadCalls))
	copy(calls, reloadCalls)
	reloadMu.Unlock()

	for _, entry := range calls {
		if entry.immediate {
			entry.fn(old, new)
		}
	}
}

func StartWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("config: cannot create watcher: %w", err)
	}

	configPath, err := configFilePath()
	if err != nil {
		w.Close()
		return err
	}

	if err := w.Add(filepath.Dir(configPath)); err != nil {
		w.Close()
		return fmt.Errorf("config: cannot watch config directory: %w", err)
	}

	watcher = w
	watcherDone = make(chan struct{})

	go func() {
		defer close(watcherDone)
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if event.Name == configPath && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
					old := Get()
					newCfg := DefaultConfig()
					if _, decodeErr := toml.DecodeFile(configPath, &newCfg); decodeErr != nil {
						log.Printf("config: hot-reload decode error: %v", decodeErr)
						continue
					}
					applyEnvOverrides(&newCfg)
					if err := validate(&newCfg); err != nil {
						log.Printf("config: hot-reload validation error: %v", err)
						continue
					}
					setCurrent(newCfg)
					notifyReload(old, newCfg)
					log.Printf("config: hot-reloaded from %s", configPath)
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Printf("config: watcher error: %v", err)
			}
		}
	}()

	return nil
}

func StopWatcher() {
	if watcher != nil {
		watcher.Close()
		<-watcherDone
		watcher = nil
	}
}

func setCurrent(cfg Config) {
	currentMu.Lock()
	defer currentMu.Unlock()
	current = cfg
}

func SafeFields() map[string]bool {
	return map[string]bool{
		"core.log_level":              true,
		"core.log_file":               true,
		"core.log_max_size":           true,
		"core.log_max_age":            true,
		"ui.theme":                    true,
		"ui.font_size":                true,
		"ui.language":                 true,
		"model.default_provider":      true,
		"model.default_model":         true,
		"model.max_tokens":            true,
		"model.timeout_s":             true,
		"model.max_concurrent":        true,
		"memory.vector_backend":       true,
		"memory.default_chunk_size":   true,
		"memory.default_chunk_overlap": true,
		"memory.auto_extract_facts":   true,
		"tools.code_exec_timeout_s":   true,
		"tools.max_concurrent_tools":  true,
		"tools.tools_network_access":  true,
		"privacy.sanitize_uploads":    true,
		"privacy.disable_telemetry":   true,
	}
}

func UnsafeFields() map[string]bool {
	return map[string]bool{
		"core.data_dir":           true,
		"server.port":             true,
		"server.host":             true,
		"server.enabled":          true,
		"server.require_auth":     true,
		"tools.sandbox_dir":       true,
		"privacy.airgap_mode":     true,
		"privacy.audit_retention_days": true,
	}
}

func applyEnvOverrides(cfg *Config) {
	prefix := "NALA_"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimPrefix(parts[0], prefix)
		val := parts[1]
		setField(cfg, strings.ToLower(key), val)
	}
}

func setField(cfg *Config, key, val string) {
	sections := strings.SplitN(key, "_", 2)
	if len(sections) < 2 {
		return
	}
	section := sections[0]
	field := sections[1]
	switch section {
	case "core":
		switch field {
		case "data_dir":
			cfg.Core.DataDir = val
		case "log_level":
			cfg.Core.LogLevel = val
		case "log_file":
			cfg.Core.LogFile = val
		case "log_max_size":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Core.LogMaxSize = v
			}
		case "log_max_age":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Core.LogMaxAge = v
			}
		case "max_sessions":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Core.MaxSessions = v
			}
		case "max_upload_size_mb":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Core.MaxUploadSizeMb = v
			}
		}
	case "server":
		switch field {
		case "enabled":
			cfg.Server.Enabled = val == "true"
		case "host":
			cfg.Server.Host = val
		case "port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Server.Port = v
			}
		case "require_auth":
			cfg.Server.RequireAuth = val == "true"
		case "cors_origins":
			cfg.Server.CorsOrigins = strings.Split(val, ",")
		case "rate_limit_rpm":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Server.RateLimitRpm = v
			}
		}
	case "model":
		switch field {
		case "default_provider":
			cfg.Model.DefaultProvider = val
		case "default_model":
			cfg.Model.DefaultModel = val
		case "max_tokens":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Model.MaxTokens = v
			}
		case "timeout_s":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Model.TimeoutS = v
			}
		case "max_concurrent":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Model.MaxConcurrent = v
			}
		case "openai_key":
			cfg.Model.OpenAIKey = val
		}
	case "memory":
		switch field {
		case "vector_backend":
			cfg.Memory.VectorBackend = val
		case "default_chunk_size":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Memory.DefaultChunkSize = v
			}
		case "default_chunk_overlap":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Memory.DefaultChunkOverlap = v
			}
		case "summarization_interval_min":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Memory.SummarizationIntervalMin = v
			}
		case "auto_extract_facts":
			cfg.Memory.AutoExtractFacts = val == "true"
		}
	case "tools":
		switch field {
		case "sandbox_dir":
			cfg.Tools.SandboxDir = val
		case "code_exec_timeout_s":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Tools.CodeExecTimeoutS = v
			}
		case "allowed_languages":
			cfg.Tools.AllowedLanguages = strings.Split(val, ",")
		case "tools_network_access":
			cfg.Tools.ToolsNetworkAccess = val == "true"
		case "max_concurrent_tools":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Tools.MaxConcurrentTools = v
			}
		}
	case "privacy":
		switch field {
		case "airgap_mode":
			cfg.Privacy.AirgapMode = val == "true"
		case "audit_retention_days":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Privacy.AuditRetentionDays = v
			}
		case "sanitize_uploads":
			cfg.Privacy.SanitizeUploads = val == "true"
		case "disable_telemetry":
			cfg.Privacy.DisableTelemetry = val == "true"
		case "session_expiry_days":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Privacy.SessionExpiryDays = v
			}
		}
	case "ui":
		switch field {
		case "theme":
			cfg.UI.Theme = val
		case "font_size":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.UI.FontSize = v
			}
		case "language":
			cfg.UI.Language = val
		}
	}
}

func validate(cfg *Config) error {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Core.LogLevel] {
		cfg.Core.LogLevel = "info"
	}
	if err := validatePort(cfg.Server.Port); err != nil {
		return err
	}
	if cfg.Core.DataDir == "" {
		return fmt.Errorf("config: core.data_dir cannot be empty")
	}
	validThemes := map[string]bool{"light": true, "dark": true, "system": true}
	if !validThemes[cfg.UI.Theme] {
		return fmt.Errorf("config: invalid ui.theme %q, must be light/dark/system", cfg.UI.Theme)
	}
	validLangs := map[string]bool{"en": true, "ja": true}
	if !validLangs[cfg.UI.Language] {
		return fmt.Errorf("config: invalid ui.language %q, must be en/ja", cfg.UI.Language)
	}
	return nil
}

func validatePort(port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("config: invalid server.port %d, must be 1024-65535", port)
	}
	return nil
}
