// Package config parses ~/.bshc/config.lua into a Config struct.
//
// The Lua config file assigns to global tables: plugins, theme, settings.
// All fields are optional; missing fields use defaults from Default().
package config

import lua "github.com/yuin/gopher-lua"

// Config holds the parsed shell configuration from Lua.
type Config struct {
	Plugins  []string
	Theme    ThemeConfig
	Settings SettingsConfig
}

// ThemeConfig controls the prompt appearance.
type ThemeConfig struct {
	PromptColor  string // hex color for the prompt text
	Background   string // hex color for the terminal background
	PromptFormat string // format string with %u/%h/%d/%t/%T/%?/%$ specifiers
}

// SettingsConfig controls shell behavior.
type SettingsConfig struct {
	HistorySize  int
	AutoComplete bool
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Theme: ThemeConfig{
			PromptColor:  "#4287f5",
			Background:   "#000000",
			PromptFormat: "[%u@%h %d]$ ",
		},
		Settings: SettingsConfig{
			HistorySize:  1000,
			AutoComplete: true,
		},
	}
}

// LoadFromLua executes a Lua config file and extracts the plugins, theme,
// and settings tables. Returns defaults for any missing table or field.
func LoadFromLua(L *lua.LState, path string) (*Config, error) {
	cfg := Default()

	if err := L.DoFile(path); err != nil {
		return cfg, err
	}

	if tbl := L.GetGlobal("plugins"); tbl != lua.LNil {
		if t, ok := tbl.(*lua.LTable); ok {
			t.ForEach(func(_, val lua.LValue) {
				if s, ok := val.(lua.LString); ok {
					cfg.Plugins = append(cfg.Plugins, string(s))
				}
			})
		}
	}

	if tbl := L.GetGlobal("theme"); tbl != lua.LNil {
		if t, ok := tbl.(*lua.LTable); ok {
			if v := t.RawGetString("prompt_color"); v != lua.LNil {
				cfg.Theme.PromptColor = string(v.(lua.LString))
			}
			if v := t.RawGetString("background"); v != lua.LNil {
				cfg.Theme.Background = string(v.(lua.LString))
			}
			if v := t.RawGetString("prompt_format"); v != lua.LNil {
				cfg.Theme.PromptFormat = string(v.(lua.LString))
			}
		}
	}

	if tbl := L.GetGlobal("settings"); tbl != lua.LNil {
		if t, ok := tbl.(*lua.LTable); ok {
			if v := t.RawGetString("history_size"); v != lua.LNil {
				if n, ok := v.(lua.LNumber); ok {
					cfg.Settings.HistorySize = int(n)
				}
			}
			if v := t.RawGetString("auto_complete"); v != lua.LNil {
				if b, ok := v.(lua.LBool); ok {
					cfg.Settings.AutoComplete = bool(b)
				}
			}
		}
	}

	return cfg, nil
}
