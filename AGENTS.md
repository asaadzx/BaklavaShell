# BakShell — AGENTS.md

## Project status

This repo is **Zen Shell** — a C++17 shell using CMake, Lua 5.3, and readline. The immediate goal is a **zero-code-change planning phase** to rebrand as **Baklava Shell** (bsh / bksh / bakshell) and rewrite in Go. No C++ code should be modified yet.

## Current architecture (C++17, for reference during migration)

- **Build system**: CMake >= 3.10, C++17 (`CMakeLists.txt` at root)
- **Dependencies**: readline, lua5.3 (pkg-config)
- **Entrypoint**: `src/main.cpp` → `zen::Shell` class
- **Source layout**: `src/{main,shell,theme}.cpp` + `include/{shell,theme}.hpp` + `zen.cpp` (monolithic legacy file, not built by CMake)
- **Config**: `~/.zencr/config.lua` — Lua table with `plugins`, `theme`, `settings`
- **Plugins**: Lua scripts in `~/.zencr/plugins/` with `execute_command(args)` and `get_prompt()` entrypoints
- **Single commit** on `main`, one branch.

## Build & run (current C++)

```sh
mkdir -p build && cd build && cmake .. && make
sudo make install   # installs to /bin/zenshell
```

Or via `install.sh` (auto-detects distro, installs deps, builds, copies to `/bin/zen`).

Run with: `zen` or `zenshell`

## Migration plan (Go rewrite)

Preserve these features in the Go rewrite:
1. **Lua-based config** at `~/.zencr/config.lua` (use gopher-lua or similar)
2. **Plugin system** with `execute_command(args)` / `get_prompt()` Lua callbacks
3. **Prompt theming** via `hex_to_ansi` + `%u`, `%h`, `%d` format strings
4. **History** (readline-style, add_history on non-empty input)
5. **Built-in commands**: `cd`, `exit`/`quit`, fallback to `fork`/`exec`
6. **SIGINT** handler that prints message instead of quitting

## Go rewrite considerations

- No C++ code changes until the Go version is ready to replace it
- Avoid vendoring the Lua C library if possible (prefer pure-Go Lua VM: gopher-lua)
- Keep the config path (`~/.zencr/`) and plugin directory structure unchanged for backward compat
- The old `zen.cpp` is dead code — ignore it
- Test Lua plugin compatibility against the 5 example plugins in `plugins/`

## Verification

No tests exist yet. When code is written:
```sh
cd build && cmake .. && make   # confirm no regressions
```
