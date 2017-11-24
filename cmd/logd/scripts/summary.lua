http = false
tick = 100

function on_tick ()
	tick = tick * 2
	config_set("tick", tick)
	print(string.format("next tick: %d", tick))
end

function on_http_error (url, method, err)
	print(err)
end

function on_log (logptr)
	-- example usage of "get" builtin
	flow = log_get(logptr, "flow")

	-- example discard of log that doesn't have flow property set
	if flow == nil then
		return nil
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

	if http then
		http_post("http://127.0.0.1:9091/qa/logging/smeagol", log_json(logptr), "application/json")
		return
	end

	print(log_string(logptr))
end

-- example usage of "config_set" builtin
-- set on_tick periodd. 0 disables the ticker
config_set("tick", tick)

-- example usage of "http_get" builtin
res, err = http_get("http://127.0.0.1:9091/server/health")

if err ~= nil then
	print(string.format("logging server not found: %s", err))
else
	print(string.format("logging server found: %s", res))
	config_set("http_concurrency", 4)
	config_set("http_channel_buffer", 20)
	http = true
end
