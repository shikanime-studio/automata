package config

import (
    "log/slog"
    "os"
    "path/filepath"

    "github.com/spf13/viper"
)

type Config struct {
    v *viper.Viper
}

// New constructs a Config, initializing defaults, binding environment variables,
// and loading optional configuration files.
func New() *Config {
    v := viper.New()
    v.SetDefault("log_level", "info")
    v.SetDefault("log_format", "text")
    v.SetDefault("log_source", false)

    v.BindEnv("log_level", "LOG_LEVEL")
    v.BindEnv("log_format", "LOG_FORMAT")
    v.BindEnv("log_source", "LOG_SOURCE")
    v.BindEnv("github_token", "GITHUB_TOKEN")

    v.AutomaticEnv()

    v.SetConfigName("automata")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    if home, err := os.UserHomeDir(); err == nil {
        v.AddConfigPath(filepath.Join(home, ".config", "automata"))
    }
    _ = v.ReadInConfig()
    return &Config{v: v}
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

// GitHubToken returns the GitHub token from config or the `GH_TOKEN`
// environment variable when not set.
func (c *Config) GitHubToken() string {
    tok := c.v.GetString("github_token")
    if tok == "" {
        tok = os.Getenv("GH_TOKEN")
    }
    return tok
}
