/**
 * SQLite database wrapper with migrations.
 * SQLiteデータベースラッパーとマイグレーションね。
 *
 * Uses modernc.org/sqlite (pure Go, no CGo) in WAL mode.
 * ピュアGoの modernc.org/sqlite を使ってWALモードで動かしてるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func New(dataDir string) (*sql.DB, error) {
	if err := ensureDir(dataDir); err != nil {
		return nil, fmt.Errorf("db: cannot create data directory: %w", err)
	}
	dbPath := filepath.Join(dataDir, "nala.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("db: cannot open database: %w", err)
	}
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-8192",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("db: pragma %q failed: %w", p, err)
		}
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
