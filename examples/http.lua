--
-- This example posts the the logs in json format to an http server listening at localhost:9091
-- This can suplied to the logd executable: logd -R examples/http.lua -f /var/log/mylog.log

local logd = require("logd")

local httpHost = "http://127.0.0.1:9091"
local httpErrors = 0
local httpReqs = 0

function logd.on_tick ()
	print(string.format("posted %d; errors %d", httpReqs, httpErrors))
end

function logd.on_http_error (url, method, err)
	httpErrors = httpErrors + 1
	print(err)
end

function logd.on_log (logptr)
	local json = logd.log_json(logptr)
	logd.http_post(string.format("%s/qa/logging/smeagol", httpHost), json, "application/json")
	httpReqs = httpReqs + 1
end

logd.config_set("tick", 100)

local res, err = logd.http_get(string.format("%s/server/health", httpHost))
if err ~= nil then
	print(string.format("logging server not found: %s", err))
else
	print(string.format("logging server found: %s", res))
	logd.config_set("http.concurrency", 4)
	logd.config_set("http.channel_buffer", 20)
end

