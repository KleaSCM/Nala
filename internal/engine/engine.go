/**
 * Application engine — orchestrates startup, dependencies, and shutdown.
 * アプリケーションエンジン — 起動と依存関係、シャットダウンを統括するの。
 *
 * Loads config, opens DB, initializes subsystems, handles graceful shutdown.
 * 設定を読み込んで、DBを開いて、各サブシステムを初期化して、グレースフルシャットダウンを処理するね。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package engine

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/KleaSCM/nala/internal/agent"
	"github.com/KleaSCM/nala/internal/config"
	"github.com/KleaSCM/nala/internal/db"
	"github.com/KleaSCM/nala/internal/logger"
	"github.com/KleaSCM/nala/internal/memory"
	"github.com/KleaSCM/nala/internal/model"
	"github.com/KleaSCM/nala/internal/tool"
)

type Engine struct {
	Config *config.Config
	Logger *logger.Logger
	DB     *sql.DB

	AgentManager     *agent.Manager
	SessionManager   *agent.SessionManager
	ConversationLoop *agent.ConversationLoop
	ModelRegistry    *model.Registry
	Router           *model.Router
	TokenTracker     *model.TokenTracker
	ToolRegistry     *tool.Registry
	MemoryManager    *memory.Manager

	cancel        context.CancelFunc
	sigWg         sync.WaitGroup
	onFatal       func(msg string)
	shutdownDelay time.Duration
}

func (e *Engine) SetOnFatal(fn func(msg string)) {
	e.onFatal = fn
}

func New() (*Engine, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("engine: config load failed: %w", err)
	}

	log, err := logger.New(cfg.Core.LogLevel, cfg.Core.LogFile, cfg.Core.LogMaxSize, cfg.Core.LogMaxAge)
	if err != nil {
		return nil, fmt.Errorf("engine: logger init failed: %w", err)
	}

	database, err := db.New(cfg.Core.DataDir)
	if err != nil {
		return nil, fmt.Errorf("engine: db init failed: %w", err)
	}

	if err := db.Migrate(database); err != nil {
		return nil, fmt.Errorf("engine: db migrate failed: %w", err)
	}

	if err := config.StartWatcher(); err != nil {
		log.Warn("config: hot-reload not available", "error", err)
	}

	modelReg := model.NewRegistry()
	toolReg := tool.NewRegistry()
	router := model.NewRouter(modelReg)
	tokenTracker := model.NewTokenTracker()
	agentMgr := agent.NewManager(database)
	sessionMgr := agent.NewSessionManager(database)
	memMgr := memory.New(database)
	loop := agent.NewConversationLoop(agentMgr, sessionMgr, modelReg, router, toolReg, tokenTracker)

	if err := modelReg.Register(model.NewOllamaProvider("")); err != nil {
		log.Warn("engine: ollama registration", "error", err)
	}
	if err := modelReg.Register(model.NewOpenAIProvider("", cfg.Model.OpenAIKey)); err != nil {
		log.Warn("engine: openai registration", "error", err)
	}

	// Set up embedding provider for memory system
	var embedder memory.Embedder
	if cfg.Model.OpenAIKey != "" {
		embedder = memory.NewCachedEmbedder(memory.NewOpenAIEmbedder("", cfg.Model.OpenAIKey, ""))
		log.Info("memory: using OpenAI embeddings")
	} else {
		ollamaEmbedder := memory.NewOllamaEmbedder("", "nomic-embed-text")
		embedder = memory.NewCachedEmbedder(ollamaEmbedder)
		log.Info("memory: using Ollama embeddings (nomic-embed-text)")
	}
	memMgr.SetEmbedder(embedder)

	// Initialize FTS
	if err := memMgr.FTS.Initialize(context.Background()); err != nil {
		log.Warn("memory: fts init", "error", err)
	}

	sandboxDir := cfg.Tools.SandboxDir
	notesDir := filepath.Join(cfg.Core.DataDir, "notes")
	os.MkdirAll(sandboxDir, 0755)
	os.MkdirAll(notesDir, 0755)

	// Wire memory tools with real functions
	memStore := tool.MemoryStore{
		StoreFn: func(ctx context.Context, fact, category string, importance float64) error {
			return memMgr.StoreUserMemory(ctx, fact, category, importance)
		},
	}
	memRecall := tool.MemoryRecall{
		RecallFn: func(ctx context.Context, query string, topK int) ([]tool.MemoryResult, error) {
			mems, err := memMgr.RecallUserMemory(ctx, query, topK)
			if err != nil {
				return nil, err
			}
			results := make([]tool.MemoryResult, len(mems))
			for i, m := range mems {
				results[i] = tool.MemoryResult{
					Fact:       m.Fact,
					Category:   m.Category,
					Confidence: m.Confidence,
					Source:     m.Source,
				}
			}
			return results, nil
		},
	}
	knowledgeSearch := tool.KnowledgeSearch{
		EmbedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			return embedder.Embed(ctx, texts)
		},
		SearchFn: func(ctx context.Context, collection string, vector []float32, topK int, minScore float64) ([]tool.VectorResult, error) {
			results, err := memMgr.VectorDB.SearchWithMinScore(ctx, collection, vector, topK, minScore)
			if err != nil {
				return nil, err
			}
			vrs := make([]tool.VectorResult, len(results))
			for i, r := range results {
				vrs[i] = tool.VectorResult{ID: r.ID, Score: r.Score, Metadata: r.Metadata}
			}
			return vrs, nil
		},
	}

	toolReg.RegisterMany(
		tool.WebSearch{},
		tool.WebFetch{},
		tool.FileRead{SandboxDir: sandboxDir},
		tool.FileWrite{SandboxDir: sandboxDir},
		tool.FileList{SandboxDir: sandboxDir},
		tool.FileDelete{SandboxDir: sandboxDir},
		tool.FileSearch{SandboxDir: sandboxDir},
		tool.ShellRun{},
		tool.CodeExecute{},
		tool.DBQuery{DB: database},
		tool.HTTPRequest{},
		tool.ImageGenerate{},
		tool.ImageAnalyze{},
		knowledgeSearch,
		memStore,
		memRecall,
		tool.CalendarList{},
		tool.CalendarCreate{},
		tool.EmailSend{},
		tool.EmailInbox{},
		tool.NotesCreate{NotesDir: notesDir},
		tool.NotesList{NotesDir: notesDir},
		tool.AURSearch{},
		tool.AURInfo{},
		tool.AURInstall{},
		tool.AURRemove{},
		tool.AURUpdate{},
		tool.AURList{},
		tool.SystemMonitor{},
		tool.SystemProcesses{},
		tool.SystemLogs{LogDir: cfg.Core.LogFile},
		tool.SystemNotify{},
	)

	return &Engine{
		Config:           cfg,
		Logger:           log,
		DB:               database,
		ModelRegistry:    modelReg,
		ToolRegistry:     toolReg,
		Router:           router,
		TokenTracker:     tokenTracker,
		AgentManager:     agentMgr,
		SessionManager:   sessionMgr,
		ConversationLoop: loop,
		MemoryManager:    memMgr,
	}, nil
}

func (e *Engine) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.Logger.Info("Nala engine started",
		"data_dir", e.Config.Core.DataDir,
		"log_level", e.Config.Core.LogLevel,
	)

	e.sigWg.Add(1)
	go func() {
		defer e.sigWg.Done()
		e.handleSignals(ctx)
	}()

	if e.SessionManager != nil {
		go e.SessionManager.CheckExpiry(ctx)
	}

	if e.MemoryManager != nil {
		go e.MemoryManager.StartConsolidationLoop(ctx)
	}

	e.Router.DiscoverModels(ctx)

	return nil
}

func (e *Engine) handleSignals(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		e.Logger.Info("signal received, starting graceful shutdown",
			"signal", sig.String(),
		)
		go e.Shutdown(30 * time.Second)
	case <-ctx.Done():
	}
}

func (e *Engine) Shutdown(timeout time.Duration) {
	e.Logger.Info("engine shutdown initiated", "timeout", timeout.String())

	done := make(chan struct{})
	go func() {
		defer close(done)

		if e.cancel != nil {
			e.cancel()
		}

		e.sigWg.Wait()

		if e.shutdownDelay > 0 {
			time.Sleep(e.shutdownDelay)
		}

		config.StopWatcher()

		if e.DB != nil {
			if err := e.DB.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "engine: db close error: %v\n", err)
			}
		}

		if e.Logger != nil {
			if err := e.Logger.Sync(); err != nil {
				fmt.Fprintf(os.Stderr, "engine: logger sync error: %v\n", err)
			}
		}
	}()

	select {
	case <-done:
		e.Logger.Info("engine shutdown complete")
	case <-time.After(timeout):
		e.Logger.Error("engine shutdown timeout, forcing exit")
		if e.onFatal != nil {
			e.onFatal("shutdown timeout")
		} else {
			os.Exit(1)
		}
	}
}
