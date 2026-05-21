package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type commandExitError struct {
	code int
}

func (e commandExitError) Error() string { return "command exited with non-zero status" }
func (e commandExitError) ExitCode() int { return e.code }

func confirmProceed(stderr io.Writer) bool {
	_, _ = fmt.Fprintf(stderr, "Proceed? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

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
