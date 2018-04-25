# Logd [![Build Status](https://travis-ci.org/ernestrc/logd.svg)](https://travis-ci.org/ernestrc/logd)
Logd is a log processor daemon that exposes a lua API to run arbitrary logic on structured logs.

## Usage
```lua
-- This can suplied to the logd executable: tail -n0 -f /var/log/mylog.log | logd -R my_script.lua

local logd = require("logd")
local os = require("os")
local tick = 100

function logd.on_tick ()
	tick = tick * 2
	logd.config_set("tick", tick)
	logd.debug({ next_tick = tick, msg = "triggered!" })
end

-- define entry point
function logd.on_log(logptr)
	logd.debug(string.format("processed log: %s", logd.log_string(logptr)))
end

-- define signal handler
function logd.on_signal(signal)
	logd.debug({ msg = string.format("My Lua script received signal: %s", signal) })

	if signal == "SIGUSR1" then
		logd.debug({ msg = "realoading after this function returns.." })
	else
		os.exit(1)
	end
end

-- example usage of "config_set" builtin
logd.config_set("tick", tick)
```

## Lua API
| Builtin | Description |
| --- | --- |
| `function logd.config_set (key, value)` | Set a configuration key/value pair. See Config table for more information. |
| `function logd.http_get (url, [headers]) body, err` | Perform a blocking HTTP GET request to `url` and return the `body` of the response and/or non-nil `err` if there was an error |
| `function logd.http_post  (url, payload, contentType [, affinity])` | Perform an HTTP POST request to the given URL with the given `payload` and `Content-Type` header set to `contentType`. Call is non-blocking unless HTTP client is applying back-pressure. `affinity` defines an HTTP queue affinity to synchronize HTTP requests. |
| `function logd.kafka_offset (key, value)` | Create a new named kafka offset. |
| `function logd.kafka_message (key, value, topic, partition [, offsetptr]) msgptr` | Create a new kafka message and return a pointer to it. `partition` can be set to -1 to use any partition. `offsetptr` can be null. |
| `function logd.kafka_produce  (msgptr)` |  Produce a single message. This is an asynchronous call that enqueues the message on the internal transmit queue, thus returning immediately unless Producer is applying back-pressure. The delivery report will be supplied via `on_kafka_report` callback if specified. |
| `function logd.log_get (logptr, key) value` | Get a property from the structured log. |
| `function logd.log_set (logptr, key, value)` | Set a property to the structured log. |
| `function logd.log_remove (logptr, key)` | Remove a property from the structured log. |
| `function logd.log_reset (logptr)` | Reset all log properties. |
| `function logd.log_string  (logptr) str` | Serialize a structured log into a string (with the same format used by the parser). |
| `function logd.log_json (logptr) str` | Serialize the structured log into a JSON string. |
| `function logd.debug (string\|table)` | Write arbitrary data to the process' debug log. |

| Hook | Description |
| --- | --- |
| `function logd.on_log (logptr)` | Logs are parsed and supplied to this handler. Use `logd.log_*` set of functions to manipulate them. |
| `function logd.on_error (logptr, error)` | When `protected` configuration is set to true, runtime errors are supplied to this handler. |
| `function logd.on_signal (signal)` | Define an OS signal handler. Note that the collector handles SIGUSR1 by default to reload script but behavior can be overwritten by this handler. |
| `function logd.on_tick ()` | Define interval handler. Interval duration can be configued via `tick` configuration. |
| `function logd.on_http_error (url, method, error)` | Define a `logd.http_post` asynchronous error handler. |
| `function logd.on_kafka_report  (msgptr, kerr)` | The delivery report callback is used by librdkafka to signal the status of a message posting, it will be called once for each message to report the status of message delivery. |

| Config | Description |
| --- | --- |
| `protected` | Run Lua code in protected mode. Runtime errors will be supplied to `logd.on_error` hook. |
| `http.concurrency` | Number of `logd.http_post` queues to instantiate. |
| `http.timeout` | `logd.http_post` timeout response timeout. |
| `http.channel_buffer` | Number of pending requests per queue before the HTTP client applies backpressure to `logd.http_post`. |
| `kafka.*` | Property passed directly to librdkafka to configure the Kafka producer. Please check https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md for more information. |
| `tick` | Interval in milliseconds to call `on_tick`. |

Lua libraries included:
- package
- table
- io
- os
- string
- bit32
- math
- debug

## Parser
The parser expects logs to be in the following format:
```
YYYY-MM-dd hh:mm:ss	LEVEL	[Thread]	Class	key: value...
```
## Build
If you do not have librdkafka (v0.11.1) installed on your system and if you have docker installed, you can use `make static` to compile and statically link a logd executable without needing to install any dependencies.

### TODO
- Add more input parsers. i.e.:
	- https://en.wikipedia.org/wiki/Common\_Log\_Format
