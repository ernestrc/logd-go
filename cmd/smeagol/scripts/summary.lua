function log (time, level, flow, operation, step, logptr)
	if flow ~= "" then
		-- make use of "get" builtin
		err = get(logptr, "err")
		if err ~= nil then
			-- example usage of "remove" builtin
			remove(logptr, "err")
			-- example usage of "set" builtin
			set(logptr, "error", err)
			print(string.format("%s\t%s\t%s\t%s: %s", time, flow, operation, step, err))
		else
			print(string.format("%s\t%s\t%s\t%s", time, flow, operation, step))
		end
	end
end
