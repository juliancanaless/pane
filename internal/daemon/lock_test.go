package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireLock_ExclusiveAccess(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// First lock should succeed
	f1, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	defer ReleaseLock(f1)

	// Second lock should fail
	f2, err := AcquireLock(lockPath)
	if err == nil {
		ReleaseLock(f2)
		t.Fatal("expected second lock to fail, but it succeeded")
	}
	if f2 != nil {
		t.Fatal("expected nil file on lock failure")
	}
}

func TestAcquireLock_ReleaseThenReacquire(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	f1, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	ReleaseLock(f1)

	// After release, should be able to acquire again
	f2, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("re-acquire after release failed: %v", err)
	}
	ReleaseLock(f2)
}

func TestAcquireLock_WritesPID(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	f, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}
	defer ReleaseLock(f)

	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	pid, err := strconv.Atoi(string(data[:len(data)-1])) // trim newline
	if err != nil {
		t.Fatalf("parse PID from lock file: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("lock file PID = %d, want %d", pid, os.Getpid())
	}
}

func TestCleanStale_NoPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "gone.pid")
	sockPath := filepath.Join(dir, "gone.sock")

	cleaned, err := CleanStale(pidPath, sockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned {
		t.Fatal("expected no cleanup when neither file exists")
	}
}

func TestCleanStale_StaleSocket(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "gone.pid")
	sockPath := filepath.Join(dir, "stale.sock")
	_ = os.WriteFile(sockPath, []byte("x"), 0o644)

	cleaned, err := CleanStale(pidPath, sockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Fatal("expected stale socket to be cleaned")
	}
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Fatal("socket file should have been removed")
	}
}

func TestCleanStale_DeadProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "stale.pid")
	sockPath := filepath.Join(dir, "stale.sock")

	// Use a PID that's almost certainly dead (max PID - 1)
	_ = os.WriteFile(pidPath, []byte("99999999\n"), 0o644)
	_ = os.WriteFile(sockPath, []byte("x"), 0o644)

	cleaned, err := CleanStale(pidPath, sockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Fatal("expected dead process state to be cleaned")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should have been removed")
	}
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Fatal("socket file should have been removed")
	}
}

func TestCleanStale_LiveProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "live.pid")
	sockPath := filepath.Join(dir, "live.sock")

	// Use our own PID — definitely alive
	_ = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)

	_, err := CleanStale(pidPath, sockPath)
	if err == nil {
		t.Fatal("expected error for live process")
	}
}
