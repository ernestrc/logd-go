--
-- This example makes use of the kafka client. Logs are serialized into JSON and produced to a test kafka topic.
-- This can be suplied to the logd executable: logd -R examples/kafka.lua -f /var/log/mylog.log
--
local kafkaoffset

kafkaHost = "localhost:9092"
kafkaTopic = "my_topic"
kafkaMsgs = 0
kafkaErrors = 0

function on_tick ()
	print(string.format("produced: %d; errors: %d", kafkaMsgs, kafkaErrors))
end

function on_kafka_report (msgptr, err)
	if err ~= nil then
		kafkaErrors = kafkaErrors + 1
		print(err)
	else
		kafkaMsgs = kafkaMsgs + 1
	end
end

function on_log (logptr) 
	if kafkaoffset ~= nil then
		-- makes use of "kafka_message" and "kafka_produce" builtins
		-- -1 partition indicates that any partition can be used
		msgptr = kafka_message("", log_json(logptr), kafkaTopic, -1, nil)
		kafka_produce(msgptr)
		return
	end
end

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
