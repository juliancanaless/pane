package session

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Environment struct {
	PaneID          string
	TTY             string
	WorkspaceRoot   string
	CWD             string
	Branch          string
	RepoID          string
	GitCommonDir    string
	ParentSessionID string
}

// DetectEnvironment describes the calling process for the daemon. Git is
// optional: outside a repository the workspace root falls back to
// PANE_WORKSPACE_ROOT or the working directory, and Branch/RepoID stay empty
// so git-scoped features degrade instead of blocking coordination.
func DetectEnvironment() (Environment, error) {
	tty := detectTTY()
	cwd, err := os.Getwd()
	if err != nil {
		return Environment{}, err
	}
	workspaceRoot := os.Getenv("PANE_WORKSPACE_ROOT")
	if workspaceRoot == "" {
		workspaceRoot, _ = gitOutput("rev-parse", "--show-toplevel")
	}
	if workspaceRoot == "" {
		workspaceRoot = cwd
	} else if absolute, absErr := filepath.Abs(workspaceRoot); absErr == nil {
		workspaceRoot = absolute
	}
	branch, _ := gitOutput("branch", "--show-current")
	repository := DetectRepository(workspaceRoot)
	return Environment{
		PaneID:          DetectPaneID(tty),
		TTY:             tty,
		WorkspaceRoot:   workspaceRoot,
		CWD:             cwd,
		Branch:          branch,
		RepoID:          repository.ID,
		GitCommonDir:    repository.GitCommonDir,
		ParentSessionID: os.Getenv("PANE_PARENT_SESSION_ID"),
	}, nil
}

func detectTTY() string {
	output, err := exec.Command("tty").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if message, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n"); message != "" {
			return "", fmt.Errorf("git %s: %s", args[0], message)
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
