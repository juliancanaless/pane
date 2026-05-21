package activity

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type WatchEvent struct {
	Path      string
	EventType EventType
	Time      time.Time
}

type WatchFunc func(WatchEvent)

type PollWatcher struct {
	Root     string
	Interval time.Duration
	OnEvent  WatchFunc
}

func (w PollWatcher) Run(ctx context.Context) error {
	interval := w.Interval
	if interval == 0 {
		interval = time.Second
	}
	snapshot, err := scan(w.Root)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := scan(w.Root)
			if err != nil {
				continue
			}
			for path, modTime := range current {
				previous, ok := snapshot[path]
				if !ok {
					w.emit(WatchEvent{Path: path, EventType: EventCreated, Time: time.Now()})
					continue
				}
				if modTime.After(previous) {
					w.emit(WatchEvent{Path: path, EventType: EventModified, Time: time.Now()})
				}
			}
			for path := range snapshot {
				if _, ok := current[path]; !ok {
					w.emit(WatchEvent{Path: path, EventType: EventDeleted, Time: time.Now()})
				}
			}
			snapshot = current
		}
	}
}

func (w PollWatcher) emit(event WatchEvent) {
	if w.OnEvent != nil {
		w.OnEvent(event)
	}
}

func scan(root string) (map[string]time.Time, error) {
	items := make(map[string]time.Time)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoredFile(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		items[path] = info.ModTime()
		return nil
	})
	return items, err
}

func ignoredDir(name string) bool {
	switch name {
	case ".git", ".internal", "bin", "dist", "node_modules", "vendor", ".next", "coverage":
		return true
	default:
		return false
	}
}

func ignoredFile(name string) bool {
	if strings.HasPrefix(name, ".") && name != ".gitignore" {
		return true
	}
	return strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm")
}
