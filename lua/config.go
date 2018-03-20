package lua

// sandboxConfig represents the configuration of a lua.Sandbox
type sandboxConfig struct {
	// general
	tick      int
	protected bool
}

/* configuration updated via builtin `config(key str, value str)`*/
const (
	luaConfigTick              = "tick"
	luaConfigProtected         = "protected"
	luaConfigHTTPConcurrency   = "http.concurrency"
	luaConfigHTTPChannelBuffer = "http.channel_buffer"
)

var availableConfigKeys = []string{
	luaConfigTick,
	luaConfigProtected,
	luaConfigHTTPConcurrency,
	luaConfigHTTPChannelBuffer,
}
