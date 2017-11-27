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
	-- example usage of "log_get" builtin
	flow = log_get(logptr, "flow")

	-- example discard log
	if flow == nil then
		return
	end

	timestamp = log_get(logptr, "timestamp")
	level = log_get(logptr, "level")
	operation = log_get(logptr, "operation")
	step = log_get(logptr, "step")
	err = log_get(logptr, "err")

	-- example usage of "log_reset" builtin
	log_reset(logptr)

	if err ~= nil then
		-- example usage of "log_set" builtin
		log_set(logptr, "err", err)
		-- example usage of "log_remove" builtin
		log_remove(logptr, "err")

		log_set(logptr, "error", err)
	end

	-- set the desired properties
	log_set(logptr, "timestamp", timestamp)
	log_set(logptr, "level", level)
	log_set(logptr, "flow", flow)
	log_set(logptr, "operation", operation)
	log_set(logptr, "step", step)

	log_set(logptr, "luaRocks", "true")

	-- makes use of "log_string" builtin
	print(log_string(logptr))
end

-- example usage of "config_set" builtin
config_set("tick", tick)