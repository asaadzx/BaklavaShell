// BakShell (bsh) — a blazing-fast, customizable shell with Lua plugin support.
//
// Usage: Run `bsh` from the terminal. Configuration lives in ~/.bshc/config.lua.
// Plugins are Lua scripts in ~/.bshc/plugins/.
package main

import (
	"fmt"
	"os"

	"bakshell/internal/shell"
)

func main() {
	s, err := shell.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(s.Run())
}
