package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/juliancanalez/pane/internal/cli"
)

type exitCoder interface {
	ExitCode() int
}

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var exitErr exitCoder
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "pane: %v\n", err)
		os.Exit(1)
	}
}
