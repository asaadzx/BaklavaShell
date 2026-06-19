package config

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Theme.PromptColor != "#4287f5" {
		t.Errorf("expected default prompt color #4287f5, got %q", cfg.Theme.PromptColor)
	}
	if cfg.Theme.Background != "#000000" {
		t.Errorf("expected default background #000000, got %q", cfg.Theme.Background)
	}
	if cfg.Theme.PromptFormat != "[%u@%h %d]$ " {
		t.Errorf("expected default prompt format, got %q", cfg.Theme.PromptFormat)
	}
	if cfg.Settings.HistorySize != 1000 {
		t.Errorf("expected history size 1000, got %d", cfg.Settings.HistorySize)
	}
	if !cfg.Settings.AutoComplete {
		t.Error("expected auto_complete true")
	}
	if cfg.Plugins != nil {
		t.Errorf("expected nil plugins, got %v", cfg.Plugins)
	}
}

func TestLoadFromLuaFull(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	content := `
plugins = {"a.lua", "b.lua"}
theme = {
    prompt_color = "#ff0000",
    background   = "#00ff00",
    prompt_format = "%u> ",
}
settings = {
    history_size = 500,
    auto_complete = false,
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromLua(L, path)
	if err != nil {
		t.Fatalf("LoadFromLua error: %v", err)
	}

	if len(cfg.Plugins) != 2 || cfg.Plugins[0] != "a.lua" || cfg.Plugins[1] != "b.lua" {
		t.Errorf("plugins = %v, want [a.lua b.lua]", cfg.Plugins)
	}
	if cfg.Theme.PromptColor != "#ff0000" {
		t.Errorf("prompt_color = %q", cfg.Theme.PromptColor)
	}
	if cfg.Theme.Background != "#00ff00" {
		t.Errorf("background = %q", cfg.Theme.Background)
	}
	if cfg.Theme.PromptFormat != "%u> " {
		t.Errorf("prompt_format = %q", cfg.Theme.PromptFormat)
	}
	if cfg.Settings.HistorySize != 500 {
		t.Errorf("history_size = %d", cfg.Settings.HistorySize)
	}
	if cfg.Settings.AutoComplete {
		t.Error("auto_complete should be false")
	}
}

func TestLoadFromLuaPartial(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	content := `plugins = {"x.lua"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromLua(L, path)
	if err != nil {
		t.Fatalf("LoadFromLua error: %v", err)
	}

	if len(cfg.Plugins) != 1 || cfg.Plugins[0] != "x.lua" {
		t.Errorf("plugins = %v", cfg.Plugins)
	}
	// Other fields should be defaults
	if cfg.Theme.PromptColor != "#4287f5" {
		t.Errorf("expected default prompt color, got %q", cfg.Theme.PromptColor)
	}
	if cfg.Settings.HistorySize != 1000 {
		t.Errorf("expected default history size, got %d", cfg.Settings.HistorySize)
	}
}

func TestLoadFromLuaMissingFile(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	_, err := LoadFromLua(L, "/nonexistent/path/config.lua")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFromLuaEmptyFile(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromLua(L, path)
	if err != nil {
		t.Fatalf("LoadFromLua error: %v", err)
	}

	// Should return defaults
	if cfg.Theme.PromptColor != "#4287f5" {
		t.Errorf("expected default prompt color, got %q", cfg.Theme.PromptColor)
	}
	if cfg.Plugins != nil {
		t.Errorf("expected nil plugins, got %v", cfg.Plugins)
	}
}

func TestLoadFromLuaWrongTypes(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	content := `
plugins = "not a table"
theme = "not a table"
settings = 42
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromLua(L, path)
	if err != nil {
		t.Fatalf("LoadFromLua error: %v", err)
	}
	// Wrong types should be silently ignored -> defaults
	if cfg.Plugins != nil {
		t.Errorf("expected nil plugins for non-table, got %v", cfg.Plugins)
	}
	if cfg.Theme.PromptColor != "#4287f5" {
		t.Errorf("expected default prompt color, got %q", cfg.Theme.PromptColor)
	}
}

func TestLoadFromLuaHistorySizeNonNumber(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	content := `settings = { history_size = "big" }`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromLua(L, path)
	if err != nil {
		t.Fatalf("LoadFromLua error: %v", err)
	}
	if cfg.Settings.HistorySize != 1000 {
		t.Errorf("expected default history size for non-number, got %d", cfg.Settings.HistorySize)
	}
}
