package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
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

func TestReleaseLock_KeepsFileSoContendersShareOneInode(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	f1, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}

	// A contender that opened the file before the holder released it. If
	// release unlinked the file, the next acquirer would create a fresh inode
	// and both could hold "the" lock at once — two daemons.
	early, err := os.OpenFile(lockPath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open contender handle: %v", err)
	}
	defer early.Close()

	ReleaseLock(f1)

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file gone after release: %v", err)
	}

	f2, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("re-acquire after release failed: %v", err)
	}
	defer ReleaseLock(f2)

	if err := syscall.Flock(int(early.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Fatal("early opener locked a second inode; two daemons could run at once")
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

func TestCleanStale_LivePaneProcess(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "live.pid")
	sockPath := filepath.Join(dir, "live.sock")

	// Use our own PID — definitely alive — and stub the identity check to
	// confirm it's a pane process, so CleanStale must refuse to clean it.
	orig := processIsPaneDaemon
	processIsPaneDaemon = func(int) bool { return true }
	defer func() { processIsPaneDaemon = orig }()

	_ = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)

	_, err := CleanStale(pidPath, sockPath)
	if err == nil {
		t.Fatal("expected error for live pane process")
	}
}

func TestCleanStale_RecycledPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "recycled.pid")
	sockPath := filepath.Join(dir, "recycled.sock")

	// PID 1 (init/launchd) is always alive but is never a pane process. This
	// simulates a reboot recycling the old daemon's PID to an unrelated
	// process — CleanStale must treat it as stale and clean up so the daemon
	// can restart, rather than reporting "daemon already running" forever.
	_ = os.WriteFile(pidPath, []byte("1\n"), 0o644)
	_ = os.WriteFile(sockPath, []byte("x"), 0o644)

	cleaned, err := CleanStale(pidPath, sockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Fatal("expected recycled PID state to be cleaned")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file should have been removed")
	}
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Fatal("socket file should have been removed")
	}
}
