-- Syntax Highlighting plugin for BakShell
-- Provides the `highlight` command to colorize command syntax

local ESC = "\27["
local RESET = ESC .. "0m"
local colors = {
  red = ESC .. "31m", green = ESC .. "32m", yellow = ESC .. "33m",
  blue = ESC .. "34m", magenta = ESC .. "35m", cyan = ESC .. "36m", gray = ESC .. "90m",
}

local patterns = {
  command = { pattern = "^%s*([%w%-_]+)",                  color = colors.cyan },
  option  = { pattern = "%-%-?[%w%-_]+",                   color = colors.yellow },
  string  = { pattern = '"[^"]*"|\'[^\']*\'',              color = colors.green },
  number  = { pattern = "%d+",                             color = colors.magenta },
  operator= { pattern = "[|<>]",                            color = colors.blue },
  variable= { pattern = "%$[%w_]+",                         color = colors.red },
  comment = { pattern = "#.*$",                            color = colors.gray },
}

local function tokenize_command(cmd)
  local tokens = {}
  local pos = 1
  while pos <= #cmd do
    local matched = false
    for _, p in pairs(patterns) do
      local match = cmd:match(p.pattern, pos)
      if match then
        table.insert(tokens, { text = match, color = p.color })
        pos = pos + #match; matched = true; break
      end
    end
    if not matched then
      table.insert(tokens, { text = cmd:sub(pos, pos), color = "" })
      pos = pos + 1
    end
  end
  return tokens
end

local function highlight_command(cmd)
  local tokens = tokenize_command(cmd)
  local out = {}
  for _, tok in ipairs(tokens) do
    table.insert(out, tok.color .. tok.text .. RESET)
  end
  return table.concat(out)
end

function execute_command(args)
  if #args == 0 then return false end
  if args[1] == "highlight" then
    local input = args[2] or ""
    print(highlight_command(input))
    return true
  end
  return false
end

print("Syntax Highlighting plugin loaded")
