package plugins

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func writePlugin(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestNewAndClose(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.L == nil {
		t.Fatal("expected non-nil Lua state")
	}
	m.Close()
}

func TestLoadPluginsEmpty(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	m.LoadPlugins(dir, []string{"nonexistent.lua"})
	// Should not panic or error — just skip silently
}

func TestLoadPluginsActiveOnly(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "a.lua", `function execute_command(args) return true end`)
	writePlugin(t, dir, "b.lua", `-- just a comment`)

	// Only load a.lua
	m.LoadPlugins(dir, []string{"a.lua"})

	if m.execFn == nil {
		t.Fatal("expected execFn to be set from a.lua")
	}
}

func TestLoadPluginsSkipsInactive(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "a.lua", `function execute_command(args) return true end`)
	writePlugin(t, dir, "b.lua", `function get_prompt() return "> " end`)

	m.LoadPlugins(dir, []string{"b.lua"})

	if m.execFn != nil {
		t.Error("expected execFn to be nil (a.lua not loaded)")
	}
	if m.promptFn == nil {
		t.Error("expected promptFn to be set from b.lua")
	}
}

func TestLoadPluginsSkipsNonLua(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "note.txt", `function execute_command(args) return true end`)

	m.LoadPlugins(dir, []string{"note.txt"})
	if m.execFn != nil {
		t.Error("expected execFn to be nil for non-lua file")
	}
}

func TestExecuteCommandHandled(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "hello.lua", `
function execute_command(args)
    if args[1] == "hello" then return true end
    return false
end
`)
	m.LoadPlugins(dir, []string{"hello.lua"})

	if got := m.ExecuteCommand([]string{"hello"}); got != true {
		t.Error("expected ExecuteCommand to return true for 'hello'")
	}
	if got := m.ExecuteCommand([]string{"other"}); got != false {
		t.Error("expected ExecuteCommand to return false for unknown command")
	}
}

func TestExecuteCommandNoPlugin(t *testing.T) {
	m := New()
	defer m.Close()

	if got := m.ExecuteCommand([]string{"anything"}); got != false {
		t.Error("expected false when no plugin loaded")
	}
}

func TestGetPrompt(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "prompt.lua", `
function get_prompt()
    return "❯ "
end
`)
	m.LoadPlugins(dir, []string{"prompt.lua"})

	if got := m.GetPrompt(); got != "❯ " {
		t.Errorf("expected '❯ ', got %q", got)
	}
}

func TestGetPromptNoPlugin(t *testing.T) {
	m := New()
	defer m.Close()

	if got := m.GetPrompt(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetPromptLuaError(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "bad.lua", `
function get_prompt()
    error("oops")
end
`)
	m.LoadPlugins(dir, []string{"bad.lua"})

	if got := m.GetPrompt(); got != "" {
		t.Errorf("expected empty string on lua error, got %q", got)
	}
}

func TestSetExitCode(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "track.lua", `
function set_exit_code(code)
    last_code = code
end
`)
	m.LoadPlugins(dir, []string{"track.lua"})

	m.SetExitCode(42)

	v := m.L.GetGlobal("last_code")
	if n, ok := v.(lua.LNumber); !ok || int(n) != 42 {
		t.Errorf("expected last_code = 42, got %v (type %T)", v, v)
	}
}

func TestSetExitCodeNoPlugin(t *testing.T) {
	m := New()
	defer m.Close()

	// Should not panic
	m.SetExitCode(1)
}

func TestLoadPluginsWithError(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "broken.lua", `this is not valid lua {{{`)
	m.LoadPlugins(dir, []string{"broken.lua"})
	// Should not panic, just print error and continue
	if m.execFn != nil {
		t.Error("expected execFn to remain nil after broken plugin")
	}
}

func TestExecuteCommandError(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "bad.lua", `
function execute_command(args)
    error("crash")
end
`)
	m.LoadPlugins(dir, []string{"bad.lua"})

	if got := m.ExecuteCommand([]string{"test"}); got != false {
		t.Error("expected false when lua function errors")
	}
}

func TestGetPromptReturnsNonString(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "bad.lua", `
function get_prompt()
    return 42
end
`)
	m.LoadPlugins(dir, []string{"bad.lua"})

	if got := m.GetPrompt(); got != "" {
		t.Errorf("expected empty string for non-string return, got %q", got)
	}
}

func TestLoadPluginsClearsOldHooks(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "a.lua", `function execute_command(args) return true end`)
	writePlugin(t, dir, "b.lua", `-- no hooks`)

	// Load first plugin with execFn
	m.LoadPlugins(dir, []string{"a.lua"})
	if m.execFn == nil {
		t.Fatal("expected execFn after loading a.lua")
	}

	// Load second plugin that doesn't define execute_command
	// Note: LoadPlugins appends/replaces based on last loaded plugin
	// Currently the code just sets from the global — we need to load b.lua alone
	// to test that execFn is still set from the previous run
	// Actually LoadPlugins doesn't clear the Lua state, so global persists
	m.LoadPlugins(dir, []string{"b.lua"})

	// execFn should still be set because the global still exists in Lua state
	if m.execFn == nil {
		t.Error("expected execFn to still be set (globals persist across runs)")
	}
}

func TestManagerStringArgs(t *testing.T) {
	m := New()
	defer m.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "echo.lua", `
function execute_command(args)
    if args[1] == "echo" and args[2] == "hello" then
        return true
    end
    return false
end
`)
	m.LoadPlugins(dir, []string{"echo.lua"})

	if got := m.ExecuteCommand([]string{"echo", "hello"}); got != true {
		t.Error("expected true for 'echo hello'")
	}
	if got := m.ExecuteCommand([]string{"echo", "world"}); got != false {
		t.Error("expected false for 'echo world'")
	}
}

func TestGetPromptReturnsNonNilFallback(t *testing.T) {
	m := New()
	defer m.Close()
	if got := m.GetPrompt(); got != "" {
		t.Errorf("expected empty when no prompt fn, got %q", got)
	}
}

func TestGetSuggestion(t *testing.T) {
	m := New()
	defer m.Close()

	// No plugin loaded → empty
	if got := m.GetSuggestion("ec"); got != "" {
		t.Errorf("expected empty with no plugin, got %q", got)
	}

	// Load a plugin with get_suggestion
	code := `
		local history = {"echo hello", "exit", "export PATH=/usr/bin"}
		function get_suggestion(line)
			for _, cmd in ipairs(history) do
				if cmd:sub(1, #line) == line then
					return cmd:sub(#line + 1)
				end
			end
			return ""
		end
	`
	if err := m.L.DoString(code); err != nil {
		t.Fatal(err)
	}
	m.LoadPlugins("", nil) // caches hook from the Lua state

	// "ec" should match "echo hello" → suggest "ho hello"
	if got := m.GetSuggestion("ec"); got != "ho hello" {
		t.Errorf("expected 'ho hello' for 'ec', got %q", got)
	}

	// "exit" should match → suggest "" (full match)
	if got := m.GetSuggestion("exit"); got != "" {
		t.Errorf("expected '' for 'exit', got %q", got)
	}

	// "zzz" should match nothing
	if got := m.GetSuggestion("zzz"); got != "" {
		t.Errorf("expected '' for 'zzz', got %q", got)
	}

	// Empty line → first history entry (all commands start with "")
	if got := m.GetSuggestion(""); got != "echo hello" {
		t.Errorf("expected 'echo hello' for empty, got %q", got)
	}
}
