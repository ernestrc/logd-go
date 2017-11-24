package lua

// Config represents the configuration of a lua.Sandbox
type Config struct {
	// general
	tick int
}

/* configuration updated via builtin `config(key str, value str)`*/
const (
	luaConfigTick              = "tick"
	luaConfigHTTPConcurrency   = "http_concurrency"
	luaConfigHTTPChannelBuffer = "http_channel_buffer"
)

var availableConfigKeys = []string{
	luaConfigTick,
	luaConfigHTTPConcurrency,
	luaConfigHTTPChannelBuffer,
}
