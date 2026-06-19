-- Command timer plugin
-- Shows elapsed time for each command with configurable threshold

local ESC = "\27["
local RESET = ESC .. "0m"
local DIM = ESC .. "2m"

local threshold_ms = 50
local start_time = 0

function execute_command(args)
  if #args == 0 then return false end
  start_time = os.clock()
  return false
end

function set_exit_code(code)
  if start_time == 0 then return end
  local elapsed = (os.clock() - start_time) * 1000
  start_time = 0

  if elapsed >= threshold_ms then
    local unit = "ms"
    local val = elapsed
    if val >= 1000 then
      val = val / 1000
      unit = "s"
    end
    io.stderr:write(string.format("%s(%.1f %s)%s\n", DIM, val, unit, RESET))
  end
end

print("Command timer plugin loaded (threshold: " .. threshold_ms .. "ms)")
