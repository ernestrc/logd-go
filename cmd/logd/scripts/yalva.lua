-- TODO add to_string function for log
--
function check_set(logptr, key)
	value = log_get(logptr, key)
	if value == nil then
		-- print(string.format("%s not set for log %s", value, to_string(logptr)))
	end
end

function on_log (logptr)
	if log_get(logptr, "flow") == "Publish" then
		check_set(logptr, "publisherId")
		check_set(logptr, "streamId")
		check_set(logptr, "operation")
		check_set(logptr, "step")
	end
	step = log_get(logptr, "step")
	if step == "Success" or step == "Failure" then
		check_set(logptr, "duration")
	end
end
