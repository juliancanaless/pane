package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectEnvironmentOutsideGitFallsBackToCwd(t *testing.T) {
	t.Setenv("PANE_WORKSPACE_ROOT", "")
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	env, err := DetectEnvironment()
	if err != nil {
		t.Fatalf("DetectEnvironment outside git: %v", err)
	}
	if env.WorkspaceRoot != cwd {
		t.Errorf("WorkspaceRoot = %q, want cwd %q", env.WorkspaceRoot, cwd)
	}
	if env.Branch != "" || env.RepoID != "" || env.GitCommonDir != "" {
		t.Errorf("git fields should be empty outside a repository, got branch=%q repo=%q common=%q", env.Branch, env.RepoID, env.GitCommonDir)
	}
}

func TestDetectEnvironmentHonorsWorkspaceRootOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PANE_WORKSPACE_ROOT", root)
	t.Chdir(t.TempDir())

	env, err := DetectEnvironment()
	if err != nil {
		t.Fatalf("DetectEnvironment with override: %v", err)
	}
	want, _ := filepath.Abs(root)
	if env.WorkspaceRoot != want {
		t.Errorf("WorkspaceRoot = %q, want override %q", env.WorkspaceRoot, want)
	}
}

func TestDetectEnvironmentInsideGitUsesToplevel(t *testing.T) {
	t.Setenv("PANE_WORKSPACE_ROOT", "")
	repoRoot, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		t.Skipf("test binary not running inside a git repository: %v", err)
	}

	env, err := DetectEnvironment()
	if err != nil {
		t.Fatalf("DetectEnvironment inside git: %v", err)
	}
	if env.WorkspaceRoot != repoRoot {
		t.Errorf("WorkspaceRoot = %q, want git toplevel %q", env.WorkspaceRoot, repoRoot)
	}
	if env.RepoID == "" {
		t.Error("RepoID should be set inside a git repository")
	}
}
