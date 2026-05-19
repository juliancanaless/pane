package cli

import (
	"os"
	"time"

	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
)

func sessionRuntime() (session.Environment, session.Manager, func(), error) {
	env, err := session.DetectEnvironment()
	if err != nil {
		return session.Environment{}, session.Manager{}, func() {}, err
	}
	dbPath, err := databasePath()
	if err != nil {
		return session.Environment{}, session.Manager{}, func() {}, err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return session.Environment{}, session.Manager{}, func() {}, err
	}
	cleanup := func() { _ = db.Close() }
	manager := session.NewManager(store.NewSessionStore(db))
	return env, manager, cleanup, nil
}

func databasePath() (string, error) {
	if value := os.Getenv("PANE_DB_PATH"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultDBPath(home), nil
}

var now = time.Now

func socketPath() (string, error) {
	if value := os.Getenv("PANE_SOCKET_PATH"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultSocketPath(home), nil
}
