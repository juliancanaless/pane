package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const DefaultDirName = ".pane"

var ErrNotFound = errors.New("not found")

func DefaultDBPath(home string) string {
	return filepath.Join(home, DefaultDirName, "pane.db")
}

func DefaultSocketPath(home string) string {
	return filepath.Join(home, DefaultDirName, "pane.sock")
}

func DefaultShimDir(home string) string {
	return filepath.Join(home, DefaultDirName, "shims")
}

func DefaultLogPath(home string) string {
	return filepath.Join(home, DefaultDirName, "logs", "pane.log")
}

func DefaultPIDPath(home string) string {
	return filepath.Join(home, DefaultDirName, "pane.pid")
}

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
