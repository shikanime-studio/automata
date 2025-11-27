package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/shikanime-studio/automata/internal/agent"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func NewCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [DIR...]",
		Short: "Run nix flake check with fix loop until success",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				rr := r
				g.Go(func() error { return runCheckLifecycle(cmd.Context(), rr) })
			}
			return g.Wait()
		},
	}
}

func runCheckLifecycle(ctx context.Context, root string) error {
	const maxIterations = 10
	var lastErr error
	for i := 1; i <= maxIterations; i++ {
		if err := agent.RunCheck(ctx, root); err == nil {
			slog.Info("check succeeded", "dir", root, "iteration", i)
			return nil
		} else {
			lastErr = err
			slog.Warn("check failed; attempting fixes", "dir", root, "iteration", i, "err", err)
		}
		if err := runUpdateFlake(ctx, root); err != nil {
			slog.WarnContext(ctx, "update flake failed", "dir", root, "err", err)
		}
		if err := runUpdateScript(ctx, root); err != nil {
			slog.WarnContext(ctx, "update script failed", "dir", root, "err", err)
		}
		if err := runUpdateSops(ctx, root); err != nil {
			slog.WarnContext(ctx, "update sops failed", "dir", root, "err", err)
		}
	}
	return lastErr
}
