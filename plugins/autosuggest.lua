-- Autosuggest plugin for BakShell
-- Provides inline command suggestions via get_suggestion hook.
-- Suggests from history first, then falls back to PATH executables.

local history = {}
local path_cmds = {}
local max_history = 1000

local function load_history()
  local f = io.open(os.getenv("HOME") .. "/.bshc/history", "r")
  if f then
    for line in f:lines() do table.insert(history, line) end
    f:close()
  end
end

local function scan_path()
  local path = os.getenv("PATH") or ""
  local dirs = {}
  for d in string.gmatch(path, "[^:]+") do
    dirs[#dirs + 1] = d
  end
  if #dirs == 0 then return end
  local cmd = "ls -1 " .. table.concat(dirs, " ") .. " 2>/dev/null"
  local handle = io.popen(cmd)
  if handle then
    for name in handle:lines() do
      path_cmds[name] = true
    end
    handle:close()
  end
end

local path_list = nil

local function get_path_list()
  if path_list then return path_list end
  path_list = {}
  for cmd in pairs(path_cmds) do
    table.insert(path_list, cmd)
  end
  table.sort(path_list)
  return path_list
end

-- Combine history + PATH into a single ordered list.
-- History items come first (most recent first), then PATH commands.
local function all_candidates()
  local seen, result = {}, {}
  for _, cmd in ipairs(history) do
    if not seen[cmd] then
      seen[cmd] = true
      result[#result + 1] = cmd
    end
  end
  for _, cmd in ipairs(get_path_list()) do
    if not seen[cmd] then
      seen[cmd] = true
      result[#result + 1] = cmd
    end
  end
  return result
end

function get_suggestion(line)
  if line == "" then return "" end
  for _, cmd in ipairs(all_candidates()) do
    if cmd:sub(1, #line) == line then
      return cmd:sub(#line + 1)
    end
  end
  return ""
end

load_history()
scan_path()

function execute_command(args)
  if #args == 0 then return false end
  local cmd = table.concat(args, " ")
  -- Deduplicate and add to front
  for i, c in ipairs(history) do
    if c == cmd then table.remove(history, i); break end
  end
  table.insert(history, 1, cmd)
  if #history > max_history then table.remove(history) end
  return false
end

print("Autosuggest loaded (" .. #history .. " history, " .. #get_path_list() .. " PATH)")
