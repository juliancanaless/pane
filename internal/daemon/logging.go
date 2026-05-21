package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	maxLogSize    = 10 * 1024 * 1024 // 10 MB
	logBackupKeep = 1                // keep 1 rotated backup
)

// SetupLogging opens the log file and redirects stdout/stderr to it.
// Rotates the log if it exceeds maxLogSize.
// Returns the file (caller should defer Close).
func SetupLogging(logPath string) (*os.File, error) {
	if logPath == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	rotateIfNeeded(logPath)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	// Redirect stdout and stderr to the log file
	os.Stdout = f
	os.Stderr = f
	return f, nil
}

func rotateIfNeeded(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxLogSize {
		return
	}
	// Rotate: move current to .1, drop anything older
	for i := logBackupKeep; i >= 1; i-- {
		src := logPath
		if i > 1 {
			src = fmt.Sprintf("%s.%d", logPath, i-1)
		}
		dst := fmt.Sprintf("%s.%d", logPath, i)
		_ = os.Remove(dst)
		_ = os.Rename(src, dst)
	}
}
