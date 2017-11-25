--
-- This example makes full use of the provided builtins. If http server is detected,
-- it will post the data there in JSON format otherwise logs are printed back to sdtout.
--
-- This is suplied to the logd binary: logd -R examples/summary.lua -f /var/log/mylog.log
--

kafkaoffset = nil
http = false
tick = 100

httpHost = "http://127.0.0.1:9091"
httpReqs = 0
httpErrors = 0

kafkaHost = "127.0.0.0:32769"
kafkaMsgs = 0
kafkaErrors = 0

function on_tick ()
	tick = tick * 2
	config_set("tick", tick)
	print("---------------------------------------------")
	print(string.format("next tick: %d", tick))
	print(string.format("successfully produced %d kafka messages to %s", kafkaMsgs, kafkaHost))
	print(string.format("%d kafka messages to %s errored", kafkaErrors, kafkaHost))
	print(string.format("posted %d HTTP payloads to %s", httpReqs, httpHost))
end

function on_kafka_report (msgptr, err)
	if err ~= nil then
		kafkaErrors = kafkaErrors + 1
		print(err)
	else
		kafkaMsgs = kafkaMsgs + 1
	end
end

function on_http_error (url, method, err)
	httpErrors = httpErrors + 1
	print(err)
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

	if kafkaoffset ~= nil then
		-- makes use of "kafka_message" and "kafka_produce" builtins
		-- -1 partition indicates that any partition can be used
		msgptr = kafka_message("", log_json(logptr), "my_topic", -1, kafkaoffset)
		kafka_produce(msgptr)
		return
	end

	if http then
		-- makes use of "http_post" and "log_json" builtins
		http_post(string.format("%s/qa/logging/smeagol", httpHost), log_json(logptr), "application/json")
		httpReqs = httpReqs + 1
		return
	end

	-- makes use of "log_string" builtin
	print(log_string(logptr))
end

-- example usage of "config_set" builtin
config_set("tick", tick)

-- example usage of "http_get" builtin
res, err = http_get(string.format("%s/server/health", httpHost))

if err ~= nil then
	print(string.format("logging server not found: %s", err))
else
	print(string.format("logging server found: %s", res))
	config_set("http.concurrency", 4)
	config_set("http.channel_buffer", 20)
	http = true
end

-- example kafka configuration
kafkaoffset, err = kafka_offset("1234")
if err ~= nil then
	print(string.format("error when creating new kafka offset: %s", err))
else
	print(string.format("created new kafka offset: %s", kafkaoffset))
	config_set("kafka.bootstrap.servers", kafkaHost)
	-- config_set("kafka.debug", "broker,topic,msg")
	config_set("kafka.socket.timeout.ms", 4000)
	config_set("kafka.group.id", "my_id")
end
