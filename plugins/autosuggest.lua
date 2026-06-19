-- Autosuggest plugin for BakShell
-- Provides history-based command suggestions via the `suggest` command

local history = {}
local max_history = 1000
local suggestion_cache = {}
local last_input = ""

local function load_history()
  local history_file = os.getenv("HOME") .. "/.bshc/history"
  local file = io.open(history_file, "r")
  if file then
    for line in file:lines() do table.insert(history, line) end
    file:close()
  end
end

local function save_history()
  local history_file = os.getenv("HOME") .. "/.bshc/history"
  local file = io.open(history_file, "w")
  if file then
    for _, cmd in ipairs(history) do file:write(cmd .. "\n") end
    file:close()
  end
end

local function add_to_history(cmd)
  for i, existing_cmd in ipairs(history) do
    if existing_cmd == cmd then table.remove(history, i); break end
  end
  table.insert(history, 1, cmd)
  if #history > max_history then table.remove(history) end
  save_history()
  suggestion_cache = {}
end

local function levenshtein_distance(s1, s2)
  local m, n = #s1, #s2
  local d = {}
  for i = 0, m do d[i] = {[0] = i} end
  for j = 0, n do d[0][j] = j end
  for j = 1, n do
    for i = 1, m do
      if s1:sub(i,i) == s2:sub(j,j) then
        d[i][j] = d[i-1][j-1]
      else
        d[i][j] = math.min(d[i-1][j] + 1, d[i][j-1] + 1, d[i-1][j-1] + 1)
      end
    end
  end
  return d[m][n]
end

local function find_suggestions(input)
  if input == last_input and suggestion_cache[input] then
    return suggestion_cache[input]
  end
  local exact, fuzzy = {}, {}
  for _, cmd in ipairs(history) do
    if cmd:sub(1, #input) == input then
      table.insert(exact, cmd)
    else
      local dist = levenshtein_distance(input, cmd:sub(1, #input))
      if dist <= 2 then table.insert(fuzzy, {cmd = cmd, dist = dist}) end
    end
  end
  table.sort(fuzzy, function(a, b) return a.dist < b.dist end)
  local suggestions = {}
  for _, c in ipairs(exact) do table.insert(suggestions, c) end
  for _, m in ipairs(fuzzy) do table.insert(suggestions, m.cmd) end
  last_input = input
  suggestion_cache[input] = suggestions
  return suggestions
end

load_history()

function execute_command(args)
  if #args == 0 then return false end
  local cmd = table.concat(args, " ")
  add_to_history(cmd)

  if args[1] == "suggest" then
    local input = args[2] or ""
    local suggestions = find_suggestions(input)
    if #suggestions > 0 then
      print("\nSuggestions:")
      for i, s in ipairs(suggestions) do
        if i > 5 then break end
        print(string.format("%d. %s", i, s))
      end
    else
      print("No suggestions found")
    end
    return true
  end
  return false
end

print("Autosuggest plugin loaded (" .. #history .. " items)")
