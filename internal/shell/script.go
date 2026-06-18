package shell

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// scriptBlock represents a parsed control flow block.
type scriptBlock struct {
	kind     string   // "cmd", "if", "for", "while"
	raw      string   // the raw condition/expression line
	body     []string // lines inside the block
	elseBody []string // for if/else
}

// parseScript splits lines into blocks, matching if/for/while with end.
func parseScript(lines []string) []scriptBlock {
	var blocks []scriptBlock
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			i++
			continue
		}

		if strings.HasPrefix(line, "if ") {
			block, next := parseBlock("if", lines, i)
			blocks = append(blocks, block)
			i = next
		} else if strings.HasPrefix(line, "for ") {
			block, next := parseBlock("for", lines, i)
			blocks = append(blocks, block)
			i = next
		} else if strings.HasPrefix(line, "while ") {
			block, next := parseBlock("while", lines, i)
			blocks = append(blocks, block)
			i = next
		} else if line == "end" || strings.HasPrefix(line, "else") {
			// orphaned end/else — skip
			i++
		} else {
			blocks = append(blocks, scriptBlock{kind: "cmd", raw: lines[i]})
			i++
		}
	}
	return blocks
}

// parseBlock reads from lines[start] until matching end, handling nested blocks.
func parseBlock(kind string, lines []string, start int) (scriptBlock, int) {
	block := scriptBlock{kind: kind, raw: strings.TrimSpace(lines[start])}
	depth := 1
	i := start + 1
	inElse := false

	for i < len(lines) && depth > 0 {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "for ") || strings.HasPrefix(line, "while ") {
			depth++
			if inElse {
				block.elseBody = append(block.elseBody, lines[i])
			} else {
				block.body = append(block.body, lines[i])
			}
		} else if line == "end" {
			depth--
			if depth > 0 {
				if inElse {
					block.elseBody = append(block.elseBody, lines[i])
				} else {
					block.body = append(block.body, lines[i])
				}
			}
		} else if line == "else" && depth == 1 && kind == "if" {
			inElse = true
		} else {
			if inElse {
				block.elseBody = append(block.elseBody, lines[i])
			} else {
				block.body = append(block.body, lines[i])
			}
		}
		i++
	}
	return block, i
}

// execBlock executes a parsed script block, returning the exit code.
func (s *Shell) execBlock(block scriptBlock) int {
	switch block.kind {
	case "cmd":
		// Check for assignment (var=value)
		if eqIdx := strings.IndexByte(block.raw, '='); eqIdx > 0 && !strings.Contains(block.raw, " ") {
			name := block.raw[:eqIdx]
			val := block.raw[eqIdx+1:]
			os.Setenv(name, val)
			return 0
		}
		// Normal command
		args := tokenize(block.raw)
		if len(args) > 0 {
			s.execute(args)
		}
		return s.lastExit

	case "if":
		cond := extractCond(block.raw, "if")
		condTrue := s.evalCondition(cond)
		if condTrue {
			s.execLines(block.body)
		} else if len(block.elseBody) > 0 {
			s.execLines(block.elseBody)
		}
		return s.lastExit

	case "for":
		varName, items := parseFor(block.raw)
		for _, item := range items {
			os.Setenv(varName, item)
			s.execLines(block.body)
		}
		return s.lastExit

	case "while":
		cond := extractCond(block.raw, "while")
		for s.evalCondition(cond) {
			s.execLines(block.body)
		}
		return s.lastExit
	}
	return 0
}

// execLines executes a list of raw script lines.
func (s *Shell) execLines(lines []string) {
	blocks := parseScript(lines)
	for _, b := range blocks {
		s.execBlock(b)
	}
}

// extractCond strips the keyword from "if cond" or "while cond".
func extractCond(raw, keyword string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, keyword+" "))
	return strings.TrimSpace(trimmed)
}

// evalCondition runs a command and returns true if exit code 0.
// For simple comparisons like "a == b", evaluate without running a command.
func (s *Shell) evalCondition(cond string) bool {
	cond = strings.TrimSpace(cond)

	// negated: ! command
	if strings.HasPrefix(cond, "! ") {
		rest := strings.TrimSpace(cond[2:])
		// Check for test-style [ expr ]
		if strings.HasPrefix(rest, "[ ") && strings.HasSuffix(rest, " ]") {
			return !s.evalTest(rest[2 : len(rest)-2])
		}
		args := tokenize(rest)
		if len(args) > 0 {
			s.execute(args)
			return s.lastExit != 0
		}
		return true
	}

	// [ expr ] style test
	if strings.HasPrefix(cond, "[ ") && strings.HasSuffix(cond, " ]") {
		expr := cond[2 : len(cond)-2]
		return s.evalTest(expr)
	}

	// Simple command
	args := tokenize(cond)
	if len(args) > 0 {
		s.execute(args)
		return s.lastExit == 0
	}
	return false
}

// evalTest evaluates test expressions like "a == b", "a != b", "-f file", etc.
func (s *Shell) evalTest(expr string) bool {
	parts := strings.Fields(expr)
	if len(parts) == 0 {
		return false
	}

	// File tests
	if len(parts) == 2 {
		switch parts[0] {
		case "-f":
			_, err := os.Stat(parts[1])
			return err == nil
		case "-d":
			info, err := os.Stat(parts[1])
			return err == nil && info.IsDir()
		case "-e":
			_, err := os.Stat(parts[1])
			return err == nil
		case "-z":
			return parts[1] == ""
		case "-n":
			return parts[1] != ""
		}
	}

	// Comparisons (a op b)
	if len(parts) == 3 {
		a := os.ExpandEnv(parts[0])
		b := os.ExpandEnv(parts[2])
		switch parts[1] {
		case "=", "==":
			return a == b
		case "!=":
			return a != b
		case "<":
			return a < b
		case ">":
			return a > b
		case "-eq":
			ai, _ := strconv.Atoi(a)
			bi, _ := strconv.Atoi(b)
			return ai == bi
		case "-ne":
			ai, _ := strconv.Atoi(a)
			bi, _ := strconv.Atoi(b)
			return ai != bi
		case "-lt":
			ai, _ := strconv.Atoi(a)
			bi, _ := strconv.Atoi(b)
			return ai < bi
		case "-gt":
			ai, _ := strconv.Atoi(a)
			bi, _ := strconv.Atoi(b)
			return ai > bi
		}
	}

	return false
}

// parseFor extracts the variable name and item list from "for var in ...".
func parseFor(raw string) (string, []string) {
	// for var in item1 item2 item3
	rest := strings.TrimPrefix(raw, "for ")
	parts := strings.Fields(rest)
	if len(parts) < 3 || parts[1] != "in" {
		return "i", []string{}
	}
	varName := parts[0]
	items := parts[2:]
	return varName, items
}

// sourceScript loads and executes a script file.
func (s *Shell) sourceScript(path string, scriptArgs []string) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source: %v\n", err)
		return 1
	}
	defer f.Close()

	// Set positional args
	s.setScriptArgs(scriptArgs)

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "source: %v\n", err)
		return 1
	}

	s.execLines(lines)
	return s.lastExit
}

// setScriptArgs sets positional argument environment variables.
func (s *Shell) setScriptArgs(args []string) {
	for i := 0; i < 10; i++ {
		name := strconv.Itoa(i)
		if i == 0 {
			os.Setenv("0", "bsh")
		} else if i-1 < len(args) {
			os.Setenv(name, args[i-1])
		} else {
			os.Setenv(name, "")
		}
	}
	os.Setenv("#", strconv.Itoa(len(args)))
}
