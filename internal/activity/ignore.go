package activity

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gitignore "github.com/sabhiram/go-gitignore"
)

// IgnoreFilter determines whether a file path should be ignored.
// It combines hardcoded rules, .gitignore, and .paneignore.
type IgnoreFilter struct {
	root     string
	mu       sync.RWMutex
	patterns *gitignore.GitIgnore
}

// NewIgnoreFilter creates a filter for the given workspace root.
// It loads .gitignore and .paneignore from the root directory.
func NewIgnoreFilter(root string) *IgnoreFilter {
	f := &IgnoreFilter{root: root}
	f.reload()
	return f
}

// ShouldIgnore returns true if the path should be excluded from file activity.
// path should be absolute.
func (f *IgnoreFilter) ShouldIgnore(path string) bool {
	// Fast-path: hardcoded directory exclusions
	rel, err := filepath.Rel(f.root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if ignoredDir(part) {
			return true
		}
	}

	// Fast-path: hardcoded file exclusions
	if ignoredFile(filepath.Base(path)) {
		return true
	}

	// Pattern-based exclusions from .gitignore / .paneignore
	f.mu.RLock()
	patterns := f.patterns
	f.mu.RUnlock()
	if patterns != nil && patterns.MatchesPath(rel) {
		return true
	}

	return false
}

// Reload re-reads .gitignore and .paneignore from the workspace root.
func (f *IgnoreFilter) Reload() {
	f.reload()
}

func (f *IgnoreFilter) reload() {
	lines := collectIgnoreLines(f.root)
	if len(lines) == 0 {
		f.mu.Lock()
		f.patterns = nil
		f.mu.Unlock()
		return
	}
	compiled := gitignore.CompileIgnoreLines(lines...)
	f.mu.Lock()
	f.patterns = compiled
	f.mu.Unlock()
}

func collectIgnoreLines(root string) []string {
	var lines []string
	for _, name := range []string{".gitignore", ".paneignore"} {
		path := filepath.Join(root, name)
		fileLines, err := readIgnoreFile(path)
		if err == nil {
			lines = append(lines, fileLines...)
		}
	}
	return lines
}

func readIgnoreFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}
