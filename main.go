package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/KleaSCM/nala/internal/engine"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	engine *engine.Engine
	ctx    context.Context
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Shutdown(ctx context.Context) {
	a.engine.Shutdown(30)
}

func (a *App) GetVersion() string {
	return "0.1.0"
}

func (a *App) GetStatus() string {
	return "ready"
}

func main() {
	e, err := engine.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	app := &App{engine: e}

	err = wails.Run(&options.App{
		Title:            "Nala",
		Width:            1200,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		StartHidden:      false,
		WindowStartState: options.Normal,
		Frameless:        false,
		Linux: &linux.Options{
			ProgramName: "nala",
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
