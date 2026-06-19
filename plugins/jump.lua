-- Directory jumping plugin (frecency-based)
-- Usage:
--   j              - list frecency-sorted directories
--   j <fragment>   - jump to most-used matching directory
--   j add [dir]    - manually add directory to database

local DATA_FILE = os.getenv("HOME") .. "/.bshc/jump.db"
local MAX_ENTRIES = 500

local db = {}
local dirty = false

local function load_db()
  local f = io.open(DATA_FILE, "r")
  if not f then return end
  for line in f:lines() do
    local dir, count, time = line:match("^([^\t]+)\t(%d+)\t(%d+)$")
    if dir then db[dir] = { count = tonumber(count), time = tonumber(time) } end
  end
  f:close()
end

local function save_db()
  if not dirty then return end
  local lines = {}
  for dir, info in pairs(db) do
    table.insert(lines, dir .. "\t" .. info.count .. "\t" .. info.time)
  end
  table.sort(lines)
  local f = io.open(DATA_FILE, "w")
  if f then f:write(table.concat(lines, "\n") .. "\n"); f:close() end
  dirty = false
end

local function add_dir(dir)
  if not dir or dir == "" then return end
  local info = db[dir]
  if info then
    info.count = info.count + 1; info.time = os.time()
  else
    if #db >= MAX_ENTRIES then
      local oldest_dir, oldest_time
      for d, i in pairs(db) do
        if not oldest_time or i.time < oldest_time then oldest_dir = d; oldest_time = i.time end
      end
      if oldest_dir then db[oldest_dir] = nil end
    end
    db[dir] = { count = 1, time = os.time() }
  end
  dirty = true
  save_db()
end

local function score(info)
  local age_hours = (os.time() - info.time) / 3600
  return info.count / (age_hours + 1)
end

local function find_matches(fragment)
  local matches = {}
  for dir, info in pairs(db) do
    if fragment == "" or dir:find(fragment, 1, true) then
      table.insert(matches, { dir = dir, score = score(info) })
    end
  end
  table.sort(matches, function(a, b) return a.score > b.score end)
  return matches
end

load_db()

function execute_command(args)
  if #args == 0 then return false end

  if args[1] == "j" then
    add_dir(io.popen("pwd"):read("*l") or "")
    if #args == 1 then
      local matches = find_matches("")
      if #matches == 0 then print("jump: no directories in database")
      else
        print("Most-used directories:")
        for i, m in ipairs(matches) do
          if i > 20 then break end
          print(string.format("%3d  %s (score: %.1f)", i, m.dir, m.score))
        end
      end
      return true
    end
    local fragment = table.concat(args, " ", 2)
    local matches = find_matches(fragment)
    if #matches == 0 then io.stderr:write("jump: no matching directory\n"); return true end
    print("cd " .. matches[1].dir)
    return true
  end

  if args[1] == "cd" and #args >= 2 then
    local target = args[2]
    if target:sub(1, 1) == "~" then target = os.getenv("HOME") .. target:sub(2) end
    add_dir(target)
    return false
  end

  return false
end

print("Jump plugin loaded (" .. #db .. " entries)")
