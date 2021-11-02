package telemetry

// Config represents settings for the telemetry tool.
type Config struct {
	// If any of these values are not provided, measurements won't be exposed.
	Username string `toml:"username"`
	Password string `toml:"password"`

	// Allow any traffic. Ie. if username/password are not specified, but AllowAny
	// is true, then the metrics endpoint will be available.
	AllowAny bool `toml:"allow_any"`

	// Allow internal private subnet traffic
	AllowInternal bool `toml:"allow_internal"`
}
