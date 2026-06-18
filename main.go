package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/KleaSCM/nala/internal/engine"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/tray.svg
var trayIcon []byte

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

func appMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	appMenu.Append(menu.SubMenu("Nala", menu.NewMenuFromItems(
		menu.Text("About Nala", nil, nil),
		menu.Separator(),
		menu.Text("Settings...", keys.CmdOrCtrl(","), nil),
		menu.Separator(),
		menu.Text("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) { os.Exit(0) }),
	)))

	appMenu.Append(menu.SubMenu("Edit", menu.NewMenuFromItems(
		menu.Text("Undo", keys.CmdOrCtrl("z"), nil),
		menu.Text("Redo", keys.CmdOrCtrl("y"), nil),
		menu.Separator(),
		menu.Text("Cut", keys.CmdOrCtrl("x"), nil),
		menu.Text("Copy", keys.CmdOrCtrl("c"), nil),
		menu.Text("Paste", keys.CmdOrCtrl("v"), nil),
		menu.Separator(),
		menu.Text("Select All", keys.CmdOrCtrl("a"), nil),
	)))

	appMenu.Append(menu.SubMenu("View", menu.NewMenuFromItems(
		menu.Text("Toggle Sidebar", keys.CmdOrCtrl("b"), nil),
		menu.Text("Toggle DevTools", keys.CmdOrCtrl("i"), nil),
		menu.Separator(),
		menu.Text("Full Screen", keys.CmdOrCtrl("f"), nil),
		menu.Text("Reload", keys.CmdOrCtrl("r"), nil),
	)))

	appMenu.Append(menu.SubMenu("Help", menu.NewMenuFromItems(
		menu.Text("Documentation", nil, nil),
		menu.Text("Report Issue", nil, nil),
		menu.Separator(),
		menu.Text("About Nala", nil, nil),
	)))

	return appMenu
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
		LogLevel:         logger.DEBUG,
		Menu:             appMenu(),
		Linux: &linux.Options{
			ProgramName: "nala",
			Icon:        trayIcon,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisablePinchZoom:     true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
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
