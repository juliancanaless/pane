package activity

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNativeWatcher_DetectsFileChanges(t *testing.T) {
	dir := resolveSymlinks(t.TempDir())

	var mu sync.Mutex
	var events []WatchEvent

	watcher := NativeWatcher{
		Root:     dir,
		Debounce: 50 * time.Millisecond,
		OnEvent: func(event WatchEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()

	// Give the watcher time to set up
	time.Sleep(200 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := len(events)
	mu.Unlock()

	if count == 0 {
		t.Fatal("expected at least one event for file creation")
	}

	// Check the event path
	mu.Lock()
	found := false
	for _, e := range events {
		if e.Path == testFile {
			found = true
			break
		}
	}
	mu.Unlock()

	if !found {
		mu.Lock()
		t.Fatalf("expected event for %s, got events: %+v", testFile, events)
		mu.Unlock()
	}
}

func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestNativeWatcher_IgnoresGitDir(t *testing.T) {
	dir := resolveSymlinks(t.TempDir())

	// Create .git directory
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []WatchEvent

	watcher := NativeWatcher{
		Root:     dir,
		Debounce: 50 * time.Millisecond,
		OnEvent: func(event WatchEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Write to .git — should be ignored
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write to src — should not be ignored
	srcDir := filepath.Join(dir, "src")
	_ = os.MkdirAll(srcDir, 0o755)
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, e := range events {
		rel, _ := filepath.Rel(dir, e.Path)
		if len(rel) >= 4 && rel[:4] == ".git" {
			t.Errorf("received event for ignored path: %s", rel)
		}
	}
}

func TestNativeWatcher_RespectsGitignore(t *testing.T) {
	dir := resolveSymlinks(t.TempDir())

	// Write .gitignore
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []WatchEvent

	watcher := NativeWatcher{
		Root:     dir,
		Debounce: 50 * time.Millisecond,
		OnEvent: func(event WatchEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Write a .log file — should be ignored
	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte("log data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a .go file — should not be ignored
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, e := range events {
		if filepath.Base(e.Path) == "app.log" {
			t.Error("received event for ignored .log file")
		}
	}

	foundGo := false
	for _, e := range events {
		if filepath.Base(e.Path) == "main.go" {
			foundGo = true
		}
	}
	if !foundGo {
		t.Error("expected event for main.go")
	}
}

func TestNativeWatcher_Debounce(t *testing.T) {
	dir := resolveSymlinks(t.TempDir())

	var mu sync.Mutex
	var events []WatchEvent

	watcher := NativeWatcher{
		Root:     dir,
		Debounce: 200 * time.Millisecond,
		OnEvent: func(event WatchEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = watcher.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Rapid writes to the same file
	testFile := filepath.Join(dir, "rapid.go")
	for i := 0; i < 10; i++ {
		_ = os.WriteFile(testFile, []byte("version "+string(rune('0'+i))), 0o644)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to flush
	time.Sleep(400 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have far fewer events than 10 writes due to debouncing
	rapidCount := 0
	for _, e := range events {
		if e.Path == testFile {
			rapidCount++
		}
	}

	if rapidCount >= 10 {
		t.Errorf("debounce should reduce events, got %d for 10 rapid writes", rapidCount)
	}
}
