function map (timestamp, level, flow, operation, step, logptr)
	-- example usage of "get" builtin
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

	set(logptr, "timestamp", timestamp)
	set(logptr, "level", level)
	set(logptr, "flow", flow)
	set(logptr, "operation", operation)
	set(logptr, "step", step)

	set(logptr, "luaRocks", "true")

	-- if return nil, log will be discarded
	return logptr
end
