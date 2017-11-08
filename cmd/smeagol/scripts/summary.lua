function map (logptr)
	-- example usage of "get" builtin
	flow = get(logptr, "flow")

	-- example discard of log that doesn't have flow property set
	if flow == nil then
		return nil
	end

	timestamp = get(logptr, "timestamp")
	level = get(logptr, "level")
	operation = get(logptr, "operation")
	step = get(logptr, "step")
	err = get(logptr, "err")
	-- example usage of "reset" builtin
	reset(logptr)

	if err ~= nil then
		-- example usage of "set" builtin
		set(logptr, "err", err)
		-- example usage of "remove" builtin
		remove(logptr, "err")

		set(logptr, "error", err)
	end

	-- set the desired properties
	set(logptr, "timestamp", timestamp)
	set(logptr, "level", level)
	set(logptr, "flow", flow)
	set(logptr, "operation", operation)
	set(logptr, "step", step)

	set(logptr, "luaRocks", "true")

	return logptr
end
