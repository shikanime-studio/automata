// Package main is the entrypoint for the Automata CLI, wiring commands and
// global configuration such as logging.
package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/shikanime-studio/automata/cmd/automata/app"
	"github.com/shikanime-studio/automata/internal/config"
)

// init configures the global logger using values from the application
// configuration.
func init() {
	cfg, err := config.New()
	if err != nil {
		slog.Error("failed to initialize config", "err", err)
		os.Exit(1)
	}
	opts := &slog.HandlerOptions{Level: cfg.LogLevel(), AddSource: cfg.LogSource()}
	var h slog.Handler
	if cfg.LogFormat() == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

// main constructs the root Cobra command, wires subcommands, and executes it.
func main() {
	rootCmd := &cobra.Command{
		Use:   "automata",
		Short: "Automata CLI",
	}
	cfg, err := config.New()
	if err != nil {
		slog.Error("failed to initialize config", "err", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(app.NewUpdateCmd(cfg))
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command execution failed", "err", err)
		os.Exit(1)
	}
}
