package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateIfNeeded_SmallFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	_ = os.WriteFile(logPath, []byte("small"), 0o644)

	rotateIfNeeded(logPath)

	// No rotation should happen
	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Fatal("should not rotate small file")
	}
}

func TestRotateIfNeeded_LargeFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create a file larger than maxLogSize
	data := make([]byte, maxLogSize+1)
	_ = os.WriteFile(logPath, data, 0o644)

	rotateIfNeeded(logPath)

	// Original should be gone (renamed to .1)
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("original log should have been rotated")
	}
	// Backup should exist
	info, err := os.Stat(logPath + ".1")
	if err != nil {
		t.Fatalf("backup log should exist: %v", err)
	}
	if info.Size() != int64(maxLogSize+1) {
		t.Fatalf("backup size = %d, want %d", info.Size(), maxLogSize+1)
	}
}

func TestRotateIfNeeded_OverwritesOldBackup(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create an old backup
	_ = os.WriteFile(logPath+".1", []byte("old"), 0o644)

	// Create a large current log
	data := make([]byte, maxLogSize+1)
	for i := range data {
		data[i] = 'x'
	}
	_ = os.WriteFile(logPath, data, 0o644)

	rotateIfNeeded(logPath)

	// Backup should be the new file, not "old"
	content, err := os.ReadFile(logPath + ".1")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if len(content) != maxLogSize+1 {
		t.Fatalf("backup should be new rotated file, got size %d", len(content))
	}
}

func TestSetupLogging_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "test.log")

	f, err := SetupLogging(logPath)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil file")
	}
	defer f.Close()

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file should exist: %v", err)
	}
}

func TestSetupLogging_EmptyPath(t *testing.T) {
	f, err := SetupLogging("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != nil {
		t.Fatal("expected nil file for empty path")
	}
}
