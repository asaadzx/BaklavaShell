# 🐚 BakShell

> **Ba**klava **Sh**ell — a blazing-fast, customizable shell with Lua plugin support, rewritten in Go.

![Go version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)
![Build](https://img.shields.io/badge/build-static-brightgreen)
[![GoDoc](https://img.shields.io/badge/go-doc-blue)](https://pkg.go.dev/github.com/anomalyco/zenshell)

Single static binary, ~3MB stripped. No runtime deps (no libreadline, no liblua).

## Quickstart

```sh
go build -ldflags="-s -w" -o bsh ./cmd/bsh
./bsh
```

## Features

- **Lua config** — theme colors, prompt format, plugin selection at `~/.bshc/config.lua`
- **Lua plugins** — overload `execute_command`, `get_prompt`, `set_exit_code` from Lua scripts
- **Aquia theme** — beautiful two-line prompt with git status, exit code, and Aquia color palette
- **Readline input** — arrow-key history, line editing, history persistence
- **Fully static** — no libreadline / liblua / CGo, no system library dependencies
- **Builtins**: `cd`, `exit`, `echo`, `pwd`, `type`, `export`, `unset`, `history`, `alias`, `unalias`, `help`
- **Data pipeline**: `from-json`, `from-csv`, `to-json`, `to-csv`, `where`, `sort-by`, `select`, `first`, `last`, `count`, `uniq`, `confirm`, `trash`, `undo`
- **Scripting**: `if`/`else`/`end`, `for`/`end`, `while`/`end`, `source`, `[ cond ]` tests

## Configuration

```lua
-- ~/.bshc/config.lua
plugins = {
    "aquia-prompt.lua",
    "autosuggest.lua",
}

theme = {
    prompt_color = "#4287f5",
    background   = "#000000",
    prompt_format = "[%u@%h %d]$ "
}

settings = {
    history_size = 1000,
    auto_complete = true
}
```

`%u` → user, `%h` → hostname, `%d` → cwd (with `~` for home).

## Plugins

Lua scripts in `~/.bshc/plugins/`. Each plugin can define:

```lua
function execute_command(args)
    if args[1] == "hello" then
        print("Hello, World!")
        return true  -- command handled
    end
    return false     -- pass to shell
end

function get_prompt()
    return "❯ "      -- custom prompt (overrides theme prompt_format)
end

function set_exit_code(code)
    -- called after every command with the exit code
end
```

### Included plugins

| Plugin | Description |
|--------|-------------|
| `aquia-prompt.lua` | Two-line prompt with git, exit code, Aquia palette |
| `git-prompt.lua` | Git branch + status in prompt |
| `autosuggest.lua` | History-based command suggestions |
| `powerlevel10k.lua` | Full-featured p10k-style prompt theme |
| `venv-prompt.lua` | Python virtualenv/conda indicator |
| `node-version.lua` | Node.js version from `.nvmrc`/`.node-version` |
| `command-timer.lua` | Elapsed time for slow commands |
| `todo.lua` | Simple todo list manager |
| `jump.lua` | Frecency-based directory jumping |
| `quote.lua` | Random developer quotes in prompt |
| `proxy.lua` | Auto proxy based on network patterns |
| `syntax-highlighting.lua` | Command syntax highlighting |

## Directory layout

```
~/.bshc/
├── config.lua         -- Shell configuration (theme, plugins, settings)
├── plugins/           -- Lua plugin scripts
├── history            -- Command history (auto-managed)
├── todos.json         -- Todo plugin data
└── jump.db            -- Jump plugin frecency database
```

## Roadmap / TODO

### High priority
- [ ] **Branding**: create logo assets, add screenshots to README
- [ ] **Suggestion plugin**: wire up `get_suggestion` hook in Go so shell can surface inline suggestions

### Medium priority
- [x] **Cleanup**: removed duplicate `ghost-prompt.lua` (identical to `powerlevel10k.lua`)
- [x] **Document `.bshc`**: directory layout documented above
- [x] **Plugin dev guide**: included in README
- [x] **Refactor plugins**: removed dead hooks (`get_prompt_suffix`, `on_command_entered`, `on_shutdown`, `on_command_complete`) not called by the shell

### Low priority
- [ ] **New plugins**: fzf integration, zoxide-style dir nav, weather, motd
- [x] **Test coverage**: added tests for config, plugins, and cmd/bsh packages

## Development

```sh
go build ./cmd/bsh && go vet ./...
go test ./...
```

Push a `v*` tag to trigger CI — builds `.tar.gz`, `.deb`, `.rpm`, `.tar.zst` for Linux and macOS via GoReleaser.

## License

MIT
