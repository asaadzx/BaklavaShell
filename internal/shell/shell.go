// Package shell implements the core BakShell REPL: input, tokenization,
// command chaining (&&/||/;/&), alias expansion, plugin dispatch, and timing.
package shell

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"bakshell/internal/config"
	"bakshell/internal/data"
	"bakshell/internal/plugins"
	"bakshell/internal/prompt"

	"github.com/chzyer/readline"
)

// Shell holds the persistent state for the shell session: config, plugins,
// environment info, aliases, the undo buffer, and the current inline suggestion.
type Shell struct {
	home        string
	cfg         *config.Config
	plugins     *plugins.Manager
	user        string
	host        string
	promptColor string
	lastExit    int
	aliases     map[string]string
	undoTable   *data.TableValue // saved table state for the 'undo' command

	suggestion         string // current inline suggestion suffix
	suggestionFor      string // the line for which the suggestion was computed
}

// New initializes the shell: determines the user/home, creates ~/.bshc/
// if missing, loads the Lua config, and starts up the plugin manager.
func New() (*Shell, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}

	s := &Shell{
		home:    home,
		user:    user,
		host:    host,
		aliases: make(map[string]string),
	}

	// Ensure ~/.bshc/ and ~/.bshc/plugins/ exist
	configDir := home + "/.bshc"
	pluginDir := configDir + "/plugins"
	for _, d := range []string{configDir, pluginDir} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			os.MkdirAll(d, 0755)
		}
	}

	// Load Lua config (falls back to defaults if missing)
	s.plugins = plugins.New()
	cfg, err := s.plugins.LoadConfig(configDir + "/config.lua")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		cfg = config.Default()
	}
	s.cfg = cfg
	s.promptColor = cfg.Theme.PromptColor

	// Load active Lua plugins
	s.plugins.LoadPlugins(pluginDir, cfg.Plugins)

	return s, nil
}

// Run enters the REPL loop: prints the prompt, reads a line, tokenizes,
// executes the pipeline, and prints command timing for slow commands (>100ms).
func (s *Shell) Run() int {
	histSize := s.cfg.Settings.HistorySize
	if histSize <= 0 {
		histSize = 1000
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "",
		HistoryFile:       s.home + "/.bshc/history",
		HistoryLimit:      histSize,
		AutoComplete:      s,
		Listener:          s,
		Painter:           s,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		return 1
	}
	defer rl.Close()
	defer s.plugins.Close()

	// Handle SIGINT gracefully (no stack trace, just a reminder)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		for range sigCh {
			fmt.Println("\nUse the 'exit' command to quit the shell.")
			rl.Refresh()
		}
	}()

	fmt.Println("Welcome to BakShell!")

	for {
		rl.SetPrompt(s.generatePrompt())

		line, err := rl.Readline()
		if err != nil {
			break // EOF (Ctrl+D) or error
		}

		// Multi-line continuation for unclosed quotes / trailing backslash
		for needsContinuation(line) {
			rl.SetPrompt("> ")
			cont, err := rl.Readline()
			if err != nil {
				break
			}
			line += "\n" + cont
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		args := tokenize(line)

		if len(args) > 0 {
			start := time.Now()
			s.execute(args)
			elapsed := time.Since(start)
			// Print dimmed timing for commands slower than 100ms
			if elapsed > 100*time.Millisecond {
				fmt.Fprintf(os.Stderr, "\033[2m(%s)\033[0m\n", elapsed.Round(time.Millisecond))
			}
		}
	}

	return 0
}

// connector represents the chaining operator between command groups.
type connector int

const (
	connSemi connector = iota // ;
	connAnd                   // &&
	connOr                    // ||
	connBg                    // &  (background)
	connEnd                   // end of input (no next group)
)

// segGroup is a group of piped commands joined by a chaining operator.
type segGroup struct {
	cmds []command
	next connector
}

// execute parses the tokenized input into groups, expands aliases, and
// runs each group with short-circuit semantics for &&/||.
func (s *Shell) execute(args []string) {
	if len(args) == 0 {
		return
	}

	args = s.expandAliases(args)
	if len(args) == 0 {
		return
	}

	groups := parseGroups(args)
	var skip bool // skip remaining groups after a failed && or successful ||

	for _, grp := range groups {
		if len(grp.cmds) == 0 {
			continue
		}

		if skip {
			skip = false
			switch grp.next {
			case connAnd:
				skip = (s.lastExit != 0)
			case connOr:
				skip = (s.lastExit == 0)
			}
			continue
		}

		exit := s.execPipeline(grp.cmds)
		s.lastExit = exit

		switch grp.next {
		case connAnd:
			skip = (exit != 0)
		case connOr:
			skip = (exit == 0)
		}
	}

	s.plugins.SetExitCode(s.lastExit)
}

// OnChange implements readline.Listener for inline suggestions.
// It recomputes the suggestion on each keystroke and accepts the
// suggestion when the right arrow key (CharForward) is pressed at
// the end of the line.
func (s *Shell) OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
	// Accept suggestion on right arrow at end of line
	if key == readline.CharForward && pos == len(line) && s.suggestion != "" {
		sug := []rune(s.suggestion)
		newLine = make([]rune, len(line)+len(sug))
		copy(newLine, line)
		copy(newLine[len(line):], sug)
		s.suggestion = ""
		return newLine, len(newLine), true
	}
	// Recompute suggestion on every meaningful keystroke
	if key != 0 {
		s.suggestion = s.plugins.GetSuggestion(string(line))
		if s.suggestion != "" {
			s.suggestionFor = string(line)
		} else {
			s.suggestionFor = ""
		}
	} else {
		s.suggestion = ""
		s.suggestionFor = ""
	}
	// Force a re-render so Paint can display the suggestion dimmed
	// (WriteRune's internal Refresh runs before the Listener is called)
	if s.suggestion != "" {
		return line, pos, true
	}
	return line, pos, false
}

// Paint implements readline.Painter for displaying inline suggestions.
// When the cursor is at the end of the line and a suggestion exists
// that matches the current input, the suggestion is rendered dimmed.
func (s *Shell) Paint(line []rune, pos int) []rune {
	if s.suggestion == "" || pos != len(line) {
		return line
	}
	// Discard stale suggestion: the current line + suggestion must still
	// produce the same full command as when it was computed
	fullLine := string(line)
	if fullLine+s.suggestion != s.suggestionFor+s.suggestion {
		return line
	}
	sug := []rune(s.suggestion)
	// Build: line + ESC[2m + suggestion + ESC[0m + cursor-back by sug width
	out := make([]rune, 0, len(line)+len(sug)+12)
	out = append(out, line...)
	out = append(out, '\033', '[', '2', 'm')
	out = append(out, sug...)
	out = append(out, '\033', '[', '0', 'm')
	w := runeWidth(sug)
	for i := 0; i < w; i++ {
		out = append(out, '\b')
	}
	return out
}

// runeWidth returns the monospace display width of runes.
func runeWidth(r []rune) int {
	// For ASCII and common Unicode, the display width is len(r) for
	// simple characters. We approximate: each rune <= 0x7F is width 1,
	// CJK range gets width 2, everything else 1.
	n := 0
	for _, c := range r {
		if c >= 0x1100 && (c <= 0x115F || c == 0x2329 || c == 0x232A ||
			(c >= 0x2E80 && c <= 0xA4CF) ||
			(c >= 0xAC00 && c <= 0xD7AF) ||
			(c >= 0xF900 && c <= 0xFAFF) ||
			(c >= 0xFE10 && c <= 0xFE19) ||
			(c >= 0xFE30 && c <= 0xFE6F) ||
			(c >= 0xFF01 && c <= 0xFF60) ||
			(c >= 0xFFE0 && c <= 0xFFE6) ||
			(c >= 0x1B000 && c <= 0x1B0FF) ||
			(c >= 0x1D300 && c <= 0x1D35F) ||
			(c >= 0x20000 && c <= 0x2A6DF) ||
			(c >= 0x2F800 && c <= 0x2FA1F)) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// isOperator reports whether a token is a shell control operator.
func isOperator(tok string) bool {
	switch tok {
	case ";", "&&", "||", "&", "|":
		return true
	}
	return false
}

// builtinNames is the set of all built-in commands (used for dispatch).
var builtinNames = map[string]bool{
	"cd": true, "exit": true, "quit": true, "echo": true,
	"pwd": true, "type": true, "export": true, "unset": true,
	"history": true, "help": true, "alias": true, "unalias": true,
	"confirm": true, "trash": true, "undo": true, "source": true,
	"from-json": true, "from-csv": true, "to-json": true, "to-csv": true,
	"where": true, "sort-by": true, "select": true,
	"first": true, "last": true, "count": true, "uniq": true, "table": true,
}

// expandAliases performs one round of alias expansion on command segments.
// Aliases are expanded at the start of each segment (after operators).
func (s *Shell) expandAliases(tokens []string) []string {
	if len(s.aliases) == 0 {
		return tokens
	}

	result := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		if i == 0 || isOperator(tokens[i-1]) {
			if expanded, ok := s.aliases[tok]; ok {
				sub := tokenize(expanded)
				if len(sub) > 0 {
					result = append(result, sub...)
				}
				continue
			}
		}
		result = append(result, tok)
	}
	return result
}

// parseGroups splits a token list into command groups separated by
// chaining operators (;, &&, ||, &).
func parseGroups(tokens []string) []segGroup {
	var groups []segGroup
	start := 0

	for i, tok := range tokens {
		var next connector
		switch tok {
		case ";":
			next = connSemi
		case "&&":
			next = connAnd
		case "||":
			next = connOr
		case "&":
			// Don't split on & if it's part of a redirect (>& or <&)
			if i > 0 && (tokens[i-1] == ">" || tokens[i-1] == "<" || tokens[i-1] == ">>") {
				continue
			}
			next = connBg
		default:
			continue
		}

		cmds := parseSegment(tokens[start:i])
		groups = append(groups, segGroup{cmds: cmds, next: next})
		start = i + 1
	}

	if start < len(tokens) {
		cmds := parseSegment(tokens[start:])
		groups = append(groups, segGroup{cmds: cmds, next: connEnd})
	}

	return groups
}

// generatePrompt builds the prompt string: either from a Lua plugin's
// get_prompt() or from the theme config with standard specifiers.
func (s *Shell) generatePrompt() string {
	if p := s.plugins.GetPrompt(); p != "" {
		// Multi-line prompts: print status lines to stdout, return only
		// the last line for readline's prompt.
		if idx := strings.LastIndex(p, "\n"); idx >= 0 {
			fmt.Print(p[:idx+1])
			return p[idx+1:]
		}
		return p
	}

	pwd, _ := os.Getwd()
	pwd = strings.Replace(pwd, s.home, "~", 1)

	return prompt.Format(s.cfg.Theme.PromptFormat, s.user, s.host, pwd, s.promptColor, s.lastExit)
}
