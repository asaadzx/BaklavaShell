package main

import (
	"os"
	"testing"
)

func TestMainImports(t *testing.T) {
	// Verify the package builds and imports correctly.
	// main() is not called directly since it creates a full shell with readline.
	// This test ensures shell.New and the module graph resolve.
	_ = os.Stdout
}

func TestBuildTag(t *testing.T) {
	// Minimal smoke test: the package name and main function exist.
	if false {
		main()
	}
}
