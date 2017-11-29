--
-- This example makes full use of the provided builtins to filter and manipulate the logs. Logs are printed back to stdout.
-- This can suplied to the logd executable: logd -R examples/summary.lua -f /var/log/mylog.log
--

kafkaoffset = nil
tick = 100

function on_tick ()
	tick = tick * 2
	config_set("tick", tick)
	print(string.format("next tick: %d", tick))
end

function on_log (logptr)
	print(log_string(logptr))
end

-- example usage of "config_set" builtin
config_set("tick", tick)
