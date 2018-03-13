--
-- This example makes use of the kafka client. Logs are serialized into JSON and produced to a test kafka topic.
-- Check https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md for more kafka configuration options.
-- This can be suplied to the logd executable: logd -R examples/kafka.lua -f /var/log/mylog.log
--
local logd = require("logd")

local kafkaoffset, err
local kafkaHost = "localhost:9092"
local kafkaTopic = "my_topic"
local kafkaMsgs = 0
local kafkaErrors = 0

function logd.on_tick ()
	print(string.format("produced: %d; errors: %d", kafkaMsgs, kafkaErrors))
end

function logd.on_kafka_report (msgptr, kerr)
	if kerr ~= nil then
		kafkaErrors = kafkaErrors + 1
		print(kerr)
	else
		kafkaMsgs = kafkaMsgs + 1
	end
end

function logd.on_log (logptr)
	if kafkaoffset ~= nil then
		-- makes use of "kafka_message" and "kafka_produce" builtins
		-- -1 partition indicates that any partition can be used
		local json = logd.log_json(logptr)
		local msgptr = logd.kafka_message("", json, kafkaTopic, -1, nil)
		logd.kafka_produce(msgptr)
		return
	end
end

logd.config_set("tick", 100)

kafkaoffset, err = logd.kafka_offset("1234")
if err ~= nil then
	print(string.format("error when creating new kafka offset: %s", err))
else
	print(string.format("created new kafka offset: %s", kafkaoffset))
	logd.config_set("kafka.go.batch.producer", false)
	logd.config_set("kafka.go.produce.channel.size", 100)
	logd.config_set("kafka.go.events.channel.size", 100)
	logd.config_set("kafka.retries", 1)
	logd.config_set("kafka.retry.backoff.ms", 1000)

	-- config_set("kafka.debug", "broker,topic,msg")
	-- config_set("kafka.delivery.report.only.error", true)
	logd.config_set("kafka.bootstrap.servers", kafkaHost)
end
