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
// environment info, aliases, and the undo buffer.
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
