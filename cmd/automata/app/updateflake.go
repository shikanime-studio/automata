package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/shikanime-studio/automata/internal/fsutil"
)

// NewUpdateFlakeCmd runs `nix flake update` for directories containing flake.nix.
func NewUpdateFlakeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "flake [DIR...]",
		Short: "Run nix flake update where flake.nix exists",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				rr := r
				g.Go(func() error { return runUpdateFlake(cmd.Context(), rr) })
			}
			return g.Wait()
		},
	}
}

// runUpdateFlake walks the directory tree and executes `nix flake update` for each found flake.nix.
func runUpdateFlake(ctx context.Context, root string) error {
	var g errgroup.Group
	handler := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "flake.nix" {
			g.Go(createFlakeUpdateJob(ctx, filepath.Dir(path)))
		}
		return nil
	}
	handler = fsutil.SkipHidden(root, handler)
	handler = fsutil.SkipGitIgnored(ctx, root, handler)
	if err := filepath.WalkDir(root, handler); err != nil {
		return fmt.Errorf("scan for flake.nix: %w", err)
	}
	return g.Wait()
}

func createFlakeUpdateJob(ctx context.Context, dir string) func() error {
	return func() error {
		slog.InfoContext(ctx, "running nix flake update", "dir", dir)
		cmd := exec.CommandContext(ctx, "nix", "flake", "update")
		cmd.Dir = dir
		cmd.Env = os.Environ()

		out, runErr := cmd.CombinedOutput()
		if len(out) > 0 {
			slog.InfoContext(ctx, "nix flake update output", "dir", dir, "output", string(out))
		}
		if runErr != nil {
			slog.WarnContext(ctx, "nix flake update failed", "dir", dir, "err", runErr)
			return fmt.Errorf("nix flake update in %s: %w", dir, runErr)
		}
		slog.InfoContext(ctx, "nix flake update completed", "dir", dir)
		return nil
	}
}
