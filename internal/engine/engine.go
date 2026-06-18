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
	"sync"
	"syscall"
	"time"

	"github.com/KleaSCM/nala/internal/config"
	"github.com/KleaSCM/nala/internal/db"
	"github.com/KleaSCM/nala/internal/logger"
)

type Engine struct {
	Config *config.Config
	Logger *logger.Logger
	DB     *sql.DB

	cancel        context.CancelFunc
	sigWg         sync.WaitGroup
	onFatal       func(msg string)
	shutdownDelay time.Duration // for testing — simulates slow cleanup
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

	return &Engine{
		Config: cfg,
		Logger: log,
		DB:     database,
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
