package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/shikanime-studio/automata/internal/fsutil"
)

// NewUpdateScriptCmd runs all update.sh scripts found under the provided directory.
func NewUpdateScriptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "updatescript [DIR...]",
		Short: "Run all update.sh scripts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				rr := r
				g.Go(func() error { return runUpdateScript(cmd.Context(), rr) })
			}
			return g.Wait()
		},
	}
}

// runUpdateScript walks the directory tree starting at root and executes every update.sh found.
func runUpdateScript(ctx context.Context, root string) error {
	var g errgroup.Group
	handler := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "update.sh" {
			g.Go(createUpdateScriptJob(ctx, path))
		}
		return nil
	}
	handler = fsutil.SkipHidden(root, handler)
	handler = fsutil.SkipGitIgnored(ctx, root, handler)
	if err := filepath.WalkDir(root, handler); err != nil {
		return fmt.Errorf("scan for update.sh: %w", err)
	}
	return g.Wait()
}

func createUpdateScriptJob(ctx context.Context, scriptPath string) func() error {
	return func() error {
		dir := filepath.Dir(scriptPath)
		slog.InfoContext(ctx, "running update script", "script", scriptPath)
		// Note: "update.sh" relies on the script being in PATH or the behavior of the shell/OS.
		// If the intention is to run the script found at scriptPath, usually one would use the absolute path or "./update.sh".
		// Preserving original behavior:
		cmd := exec.CommandContext(ctx, "update.sh")
		cmd.Dir = dir
		cmd.Env = os.Environ()

		out, runErr := cmd.CombinedOutput()
		if len(out) > 0 {
			slog.InfoContext(ctx, "update.sh output", "script", scriptPath, "output", string(out))
		}
		if runErr != nil {
			slog.WarnContext(ctx, "update.sh failed", "script", scriptPath, "err", runErr)
			return fmt.Errorf("run %s: %w", scriptPath, runErr)
		}
		slog.InfoContext(ctx, "update script completed", "script", scriptPath)
		return nil
	}
}
