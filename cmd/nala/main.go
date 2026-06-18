/**
 * Desktop application entry point for Nala.
 * Nalaのデスクトップアプリケーションエントリポイントね。
 *
 * Initializes the engine and starts the Wails window.
 * エンジンを初期化して、Wailsウィンドウを起動するの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package main

import (
	"fmt"
	"os"

	"github.com/KleaSCM/nala/internal/engine"
)

func main() {
	e, err := engine.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	if err := e.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	select {}
}
