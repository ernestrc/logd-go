function log (time, level, flow, operation, step, properties)
	if flow ~= "" then
		print(string.format("%s\t%s\t%s\t%s", time, flow, operation, step))
	end
end
