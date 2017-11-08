function log (timestamp, level, flow, operation, step, logptr)
	if flow ~= "" then
		-- make use of "get" builtin
		err = get(logptr, "err")
		-- make use of "reset" builtin
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
	end
end
