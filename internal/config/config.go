package config

import (
	"log/slog"

	"github.com/spf13/viper"
)

// Config wraps application configuration and environment bindings.
type Config struct{ v *viper.Viper }

// New constructs a new Config with defaults and environment bindings.
func New() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("log_source", false)

	if err := v.BindEnv("log_level", "LOG_LEVEL"); err != nil {
		return nil, err
	}
	if err := v.BindEnv("log_format", "LOG_FORMAT"); err != nil {
		return nil, err
	}
	if err := v.BindEnv("log_source", "LOG_SOURCE"); err != nil {
		return nil, err
	}
	if err := v.BindEnv("github_token", "GITHUB_TOKEN"); err != nil {
		return nil, err
	}

	return &Config{v: v}, nil
}

// LogLevel returns the configured slog level, defaulting to info when unset or
// unknown.
func (c *Config) LogLevel() slog.Level {
	switch c.v.GetString("log_level") {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LogFormat returns the desired log format ("json" or "text"), falling back to
// "text" for unknown values.
func (c *Config) LogFormat() string {
	switch c.v.GetString("log_format") {
	case "json":
		return "json"
	case "text":
		return "text"
	default:
		return "text"
	}
}

// LogSource reports whether log records should include source location (file and
// line).
func (c *Config) LogSource() bool {
	return c.v.GetBool("log_source")
}

// GitHubToken returns the GitHub token from config.
func (c *Config) GitHubToken() string {
	return c.v.GetString("github_token")
}
