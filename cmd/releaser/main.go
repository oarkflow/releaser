/*
Package main provides the CLI entry point for Releaser.
*/
package main

import (
	"os"

	"github.com/oarkflow/releaser/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
