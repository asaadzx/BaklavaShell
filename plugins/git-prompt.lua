-- Git Prompt plugin for BakShell
-- Shows Git branch name and status as the prompt

local ESC = "\27["
local RESET = ESC .. "0m"
local colors = {
  red = ESC .. "31m",
  green = ESC .. "32m",
  yellow = ESC .. "33m",
  cyan = ESC .. "36m",
  magenta = ESC .. "35m",
}

local symbols = {
  modified = "●",
  staged = "●",
  untracked = "?",
  ahead = "↑",
  behind = "↓",
  diverged = "↕",
  stashed = "⚑",
}

local status_cache = {}
local cache_timeout = 2
local last_update = 0

local function get_git_branch()
  local handle = io.popen("git rev-parse --abbrev-ref HEAD 2>/dev/null")
  if not handle then return nil end
  local branch = handle:read("*a"):gsub("%s+$", "")
  handle:close()
  return branch ~= "" and branch or nil
end

local function get_git_status()
  local current_time = os.time()
  if current_time - last_update < cache_timeout then
    return status_cache
  end

  local status = {
    branch = get_git_branch(),
    modified = false, staged = false, untracked = false,
    ahead = 0, behind = 0, stashed = false,
  }

  if not status.branch then return nil end

  local handle = io.popen("git status --porcelain 2>/dev/null")
  if handle then
    for line in handle:lines() do
      if line:match("^%s*[AMDR]") then status.staged = true
      elseif line:match("^%s*[MT]") then status.modified = true
      elseif line:match("^%s*%?%?") then status.untracked = true end
    end
    handle:close()
  end

  handle = io.popen("git stash list 2>/dev/null")
  if handle then status.stashed = handle:read("*a") ~= ""; handle:close() end

  handle = io.popen("git rev-list --count --left-right @{upstream}...HEAD 2>/dev/null")
  if handle then
    local behind, ahead = handle:read("*n", "*n")
    if behind and ahead then status.behind = behind; status.ahead = ahead end
    handle:close()
  end

  status_cache = status
  last_update = current_time
  return status
end

function get_prompt()
  local status = get_git_status()
  if not status then return "$ " end

  local parts = { colors.cyan .. status.branch .. RESET }

  if status.modified then table.insert(parts, colors.red .. symbols.modified .. RESET) end
  if status.staged then table.insert(parts, colors.green .. symbols.staged .. RESET) end
  if status.untracked then table.insert(parts, colors.yellow .. symbols.untracked .. RESET) end

  if status.ahead > 0 and status.behind > 0 then
    table.insert(parts, colors.magenta .. symbols.diverged .. RESET)
  elseif status.ahead > 0 then
    table.insert(parts, colors.green .. symbols.ahead .. RESET)
  elseif status.behind > 0 then
    table.insert(parts, colors.red .. symbols.behind .. RESET)
  end

  if status.stashed then table.insert(parts, colors.yellow .. symbols.stashed .. RESET) end

  return table.concat(parts, " ") .. " $ "
end

function execute_command(args)
  if #args == 0 then return false end
  if args[1] == "git-status" then
    local status = get_git_status()
    if not status then print("Not a Git repository"); return true end
    print("Branch: " .. status.branch)
    print("  Modified: " .. (status.modified and "Yes" or "No"))
    print("  Staged: " .. (status.staged and "Yes" or "No"))
    print("  Untracked: " .. (status.untracked and "Yes" or "No"))
    print("  Ahead: " .. status.ahead)
    print("  Behind: " .. status.behind)
    return true
  end
  return false
end

print("Git Prompt plugin loaded")
