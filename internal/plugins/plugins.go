// Package plugins manages Lua plugin scripts for BakShell.
//
// Plugins are loaded from ~/.bshc/plugins/*.lua. Each plugin can define
// any of these global functions that the shell calls:
//   - execute_command(args) — return true to claim the command, false to pass through
//   - get_prompt() — return a string to override the prompt
//   - set_exit_code(code) — called after every command with its exit code
//   - get_suggestion(line) — return a completion suffix for inline autosuggest
package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bakshell/internal/config"

	lua "github.com/yuin/gopher-lua"
)

// Manager owns the Lua VM and caches references to active plugin hooks.
type Manager struct {
	L        *lua.LState
	execFn   *lua.LFunction // cached execute_command, or nil
	promptFn *lua.LFunction // cached get_prompt, or nil
	suggestFn *lua.LFunction // cached get_suggestion, or nil
}

// New creates a plugin manager with a fresh Lua VM.
func New() *Manager {
	return &Manager{L: lua.NewState()}
}

// Close shuts down the Lua VM.
func (m *Manager) Close() {
	m.L.Close()
}

// LoadConfig delegates to config.LoadFromLua using the internal Lua state.
func (m *Manager) LoadConfig(path string) (*config.Config, error) {
	cfg, err := config.LoadFromLua(m.L, path)
	if err != nil {
		return cfg, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// LoadPlugins reads the plugin directory and runs each active .lua file.
// After loading, it caches plugin hooks from the Lua state.
func (m *Manager) LoadPlugins(pluginDir string, active []string) {
	if pluginDir != "" {
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read plugins directory: %v\n", err)
		} else {
			activeSet := make(map[string]bool, len(active))
			for _, p := range active {
				activeSet[p] = true
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
					continue
				}
				if !activeSet[entry.Name()] {
					continue
				}

				path := filepath.Join(pluginDir, entry.Name())
				if err := m.L.DoFile(path); err != nil {
					fmt.Fprintf(os.Stderr, "Error loading plugin %s: %v\n", entry.Name(), err)
					continue
				}
				fmt.Printf("Loaded plugin: %s\n", entry.Name())
			}
		}
	}

	// Cache plugin hooks — only the last plugin's definitions take effect
	if fn := m.L.GetGlobal("execute_command"); fn != lua.LNil {
		if f, ok := fn.(*lua.LFunction); ok {
			m.execFn = f
		}
	}
	if fn := m.L.GetGlobal("get_prompt"); fn != lua.LNil {
		if f, ok := fn.(*lua.LFunction); ok {
			m.promptFn = f
		}
	}
	if fn := m.L.GetGlobal("get_suggestion"); fn != lua.LNil {
		if f, ok := fn.(*lua.LFunction); ok {
			m.suggestFn = f
		}
	}
}

// ExecuteCommand calls the cached execute_command hook with args as a Lua table.
// Returns true if the plugin claimed the command.
func (m *Manager) ExecuteCommand(args []string) bool {
	if m.execFn == nil {
		return false
	}

	tbl := m.L.NewTable()
	for i, a := range args {
		tbl.RawSetInt(i+1, lua.LString(a))
	}

	if err := m.L.CallByParam(lua.P{
		Fn:      m.execFn,
		NRet:    1,
		Protect: true,
	}, tbl); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing Lua command handler: %v\n", err)
		return false
	}

	ret := m.L.Get(-1)
	m.L.Pop(1)
	return ret == lua.LTrue
}

// GetPrompt calls the cached get_prompt hook and returns its string result.
// Returns "" if no plugin sets get_prompt or if it errors.
func (m *Manager) GetPrompt() string {
	if m.promptFn == nil {
		return ""
	}

	if err := m.L.CallByParam(lua.P{
		Fn:      m.promptFn,
		NRet:    1,
		Protect: true,
	}); err != nil {
		return ""
	}

	ret := m.L.Get(-1)
	m.L.Pop(1)
	if s, ok := ret.(lua.LString); ok {
		return string(s)
	}
	return ""
}

// SetExitCode calls the cached set_exit_code hook with the given code.
// It is safe to call even if no plugin defines set_exit_code.
func (m *Manager) SetExitCode(code int) {
	var fn *lua.LFunction
	if v := m.L.GetGlobal("set_exit_code"); v != lua.LNil {
		fn, _ = v.(*lua.LFunction)
	}
	if fn == nil {
		return
	}
	_ = m.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, lua.LNumber(code))
}

// GetSuggestion calls the cached get_suggestion hook with the current line
// and returns the suggested completion text. Returns "" if no suggestion.
func (m *Manager) GetSuggestion(line string) string {
	if m.suggestFn == nil {
		return ""
	}
	if err := m.L.CallByParam(lua.P{
		Fn:      m.suggestFn,
		NRet:    1,
		Protect: true,
	}, lua.LString(line)); err != nil {
		return ""
	}
	ret := m.L.Get(-1)
	m.L.Pop(1)
	if s, ok := ret.(lua.LString); ok {
		return string(s)
	}
	return ""
}
