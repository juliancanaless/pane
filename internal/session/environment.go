package session

import (
	"bytes"
	"os"
	"os/exec"
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

func DetectEnvironment() (Environment, error) {
	tty := detectTTY()
	workspaceRoot, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return Environment{}, err
	}
	branch, _ := gitOutput("branch", "--show-current")
	repository := DetectRepository(workspaceRoot)
	cwd, err := os.Getwd()
	if err != nil {
		return Environment{}, err
	}
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
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
