// Package prompt formats prompt strings using the theme config.
//
// Supported prompt specifiers:
//
//	%u — username
//	%h — hostname
//	%d — current directory (~ for home)
//	%t — current time (HH:MM)
//	%T — current time (HH:MM:SS)
//	%? — last exit code
//	%$ — "#" for root, "$" for normal users
package prompt

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// HexToANSI converts a hex color string (e.g. "#4287f5") to an ANSI 24-bit
// foreground escape sequence. Returns reset code on invalid input.
func HexToANSI(hex string) string {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return "\033[0m"
	}

	var r, g, b int
	if _, err := fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b); err != nil {
		return "\033[0m"
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// Format replaces specifiers in format with their runtime values and wraps
// the result in the given color. Uses strings.ReplaceAll (no regex in hot path).
func Format(format, user, host, cwd, colorHex string, lastExit int) string {
	if format == "" {
		format = "[%u@%h %d]$ "
	}

	now := time.Now()

	prompt := format
	prompt = strings.ReplaceAll(prompt, "%u", user)
	prompt = strings.ReplaceAll(prompt, "%h", host)
	prompt = strings.ReplaceAll(prompt, "%d", cwd)
	prompt = strings.ReplaceAll(prompt, "%t", now.Format("15:04"))
	prompt = strings.ReplaceAll(prompt, "%T", now.Format("15:04:05"))

	if lastExit == 0 {
		prompt = strings.ReplaceAll(prompt, "%?", "0")
	} else {
		prompt = strings.ReplaceAll(prompt, "%?", fmt.Sprintf("%d", lastExit))
	}

	root := "$"
	if os.Geteuid() == 0 {
		root = "#"
	}
	prompt = strings.ReplaceAll(prompt, "%$", root)

	color := HexToANSI(colorHex)
	return color + prompt + "\033[0m"
}
