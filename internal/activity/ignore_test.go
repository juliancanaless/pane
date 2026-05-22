package activity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreFilter_HardcodedDirs(t *testing.T) {
	dir := t.TempDir()
	filter := NewIgnoreFilter(dir)

	cases := []struct {
		rel  string
		want bool
	}{
		{".git/config", true},
		{"node_modules/foo/bar.js", true},
		{"vendor/pkg/mod.go", true},
		{".next/cache/data", true},
		{"src/main.go", false},
		{"internal/auth/auth.go", false},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.rel)
		got := filter.ShouldIgnore(path)
		if got != tc.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}
}

func TestIgnoreFilter_HardcodedFiles(t *testing.T) {
	dir := t.TempDir()
	filter := NewIgnoreFilter(dir)

	cases := []struct {
		rel  string
		want bool
	}{
		{".DS_Store", true},
		{".swp", true},
		{"data.db", true},
		{"data.db-wal", true},
		{"main.go", false},
		{".gitignore", false},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.rel)
		got := filter.ShouldIgnore(path)
		if got != tc.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}
}

func TestIgnoreFilter_GitignorePatterns(t *testing.T) {
	dir := t.TempDir()

	// Write a .gitignore
	gitignore := "*.log\nbuild/\ntmp/**\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatal(err)
	}

	filter := NewIgnoreFilter(dir)

	cases := []struct {
		rel  string
		want bool
	}{
		{"app.log", true},
		{"logs/debug.log", true},
		{"build/output.bin", true},
		{"tmp/cache/data", true},
		{"src/main.go", false},
		{"README.md", false},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.rel)
		got := filter.ShouldIgnore(path)
		if got != tc.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}
}

func TestIgnoreFilter_PaneignorePatterns(t *testing.T) {
	dir := t.TempDir()

	// Write a .paneignore
	paneignore := "*.generated.go\nfixtures/\n"
	if err := os.WriteFile(filepath.Join(dir, ".paneignore"), []byte(paneignore), 0o644); err != nil {
		t.Fatal(err)
	}

	filter := NewIgnoreFilter(dir)

	cases := []struct {
		rel  string
		want bool
	}{
		{"api/types.generated.go", true},
		{"fixtures/testdata.json", true},
		{"src/main.go", false},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.rel)
		got := filter.ShouldIgnore(path)
		if got != tc.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.rel, got, tc.want)
		}
	}
}

func TestIgnoreFilter_CombinedGitignoreAndPaneignore(t *testing.T) {
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".paneignore"), []byte("*.snap\n"), 0o644)

	filter := NewIgnoreFilter(dir)

	// .gitignore pattern
	if !filter.ShouldIgnore(filepath.Join(dir, "app.log")) {
		t.Error("expected .log to be ignored (from .gitignore)")
	}
	// .paneignore pattern
	if !filter.ShouldIgnore(filepath.Join(dir, "test.snap")) {
		t.Error("expected .snap to be ignored (from .paneignore)")
	}
	// Neither
	if filter.ShouldIgnore(filepath.Join(dir, "main.go")) {
		t.Error("expected .go to not be ignored")
	}
}

func TestIgnoreFilter_NoIgnoreFiles(t *testing.T) {
	dir := t.TempDir()
	filter := NewIgnoreFilter(dir)

	// Should still work with hardcoded rules
	if !filter.ShouldIgnore(filepath.Join(dir, "node_modules/foo.js")) {
		t.Error("expected node_modules to be ignored even without .gitignore")
	}
	if filter.ShouldIgnore(filepath.Join(dir, "src/main.go")) {
		t.Error("expected src/main.go to not be ignored")
	}
}

func TestIgnoreFilter_Reload(t *testing.T) {
	dir := t.TempDir()
	filter := NewIgnoreFilter(dir)

	// Initially no .gitignore — *.log not ignored
	if filter.ShouldIgnore(filepath.Join(dir, "app.log")) {
		t.Error("expected .log to not be ignored before .gitignore exists")
	}

	// Write .gitignore
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644)
	filter.Reload()

	// Now it should be ignored
	if !filter.ShouldIgnore(filepath.Join(dir, "app.log")) {
		t.Error("expected .log to be ignored after reload")
	}
}
