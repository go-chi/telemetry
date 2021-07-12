package telemetry

// Config represents settings for the telemetry tool.
type Config struct {
	// If any of these values are not provided, measurements won't be exposed.
	Username string `toml:"username"`
	Password string `toml:"password"`
}
