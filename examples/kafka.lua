--
-- This example makes use of the kafka client. Logs are serialized into JSON and produced to a test kafka topic.
-- Check https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md for more kafka configuration options.
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

config_set("tick", 100)

kafkaoffset, err = kafka_offset("1234")

if err ~= nil then
	print(string.format("error when creating new kafka offset: %s", err))
else
	print(string.format("created new kafka offset: %s", kafkaoffset))
	config_set("kafka.go.batch.producer", false)
	config_set("kafka.go.produce.channel.size", 0)
	config_set("kafka.go.events.channel.size", 0)

	config_set("kafka.bootstrap.servers", kafkaHost)

	-- config_set("kafka.debug", "broker,topic,msg")
	config_set("kafka.retries", 0)
	config_set("kafka.retry.backoff.ms", 100)
	-- config_set("kafka.delivery.report.only.error", true)
end
