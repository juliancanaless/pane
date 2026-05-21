package cli

import (
	"io"
	"os"
	"os/exec"
)

type commandExitError struct {
	code int
}

func (e commandExitError) Error() string { return "command exited with non-zero status" }
func (e commandExitError) ExitCode() int { return e.code }

func runRealGit(args []string, stdout, stderr io.Writer) (int, error) {
	git := os.Getenv("PANE_REAL_GIT")
	if git == "" {
		git = "git"
	}
	cmd := exec.Command(git, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}
