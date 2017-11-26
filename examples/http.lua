--
-- This example posts the the logs in json format to an http server listening at localhost:9091
-- This can suplied to the logd executable: logd -R examples/http.lua -f /var/log/mylog.log

httpHost = "http://127.0.0.1:9091"
httpErrors = 0
httpReqs = 0

function on_tick ()
	print(string.format("posted %d; errors %d", httpReqs, httpErrors))
end

function on_http_error (url, method, err)
	httpErrors = httpErrors + 1
	print(err)
end

function on_log (logptr) 
	http_post(string.format("%s/qa/logging/smeagol", httpHost), log_json(logptr), "application/json")
	httpReqs = httpReqs + 1
end

config_set("tick", 100)

res, err = http_get(string.format("%s/server/health", httpHost))
if err ~= nil then
	print(string.format("logging server not found: %s", err))
else
	print(string.format("logging server found: %s", res))
	config_set("http.concurrency", 4)
	config_set("http.channel_buffer", 20)
end

