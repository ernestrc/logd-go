--
-- This example makes full use of the provided builtins to filter and manipulate the logs.
-- Logs are printed back to stdout.
-- This can suplied to the logd executable: logd -R examples/summary.lua -f /var/log/mylog.log
--
local logd = require("logd")
local tick = 100

function logd.on_tick ()
	local ntick = tick * 2
	logd.config_set("tick", ntick)

	logd.debug({ next_tick = ntick, msg = "triggered!" })
end

function logd.on_log(logptr)
	logd.debug(string.format("processed log: %s", logd.log_string(logptr)))
end

-- example usage of "config_set" builtin
logd.config_set("tick", tick)
