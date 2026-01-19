package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/goldyfruit/elemental-node-mapper/cmd"
	"github.com/goldyfruit/elemental-node-mapper/internal/exit"
)

func main() {
	root := cmd.NewRootCmd()
	if err := root.Execute(); err != nil {
		exitCode := 1
		var exitErr *exit.Error
		if errors.As(err, &exitErr) {
			exitCode = exitErr.Code
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitCode)
	}
}
