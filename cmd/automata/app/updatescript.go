package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/fsutil"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
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
	var scripts []string
	err := fsutil.WalkDirWithGitignore(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "update.sh" {
			scripts = append(scripts, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan for update.sh: %w", err)
	}

	if len(scripts) == 0 {
		slog.InfoContext(ctx, "no update.sh scripts found", "root", root)
		return nil
	}
	var g errgroup.Group
	for _, script := range scripts {
		s := script
		g.Go(func() error {
			dir := filepath.Dir(s)
			slog.InfoContext(ctx, "running update script", "script", s)
			cmd := exec.CommandContext(ctx, "update.sh")
			cmd.Dir = dir
			cmd.Env = os.Environ()

			out, runErr := cmd.CombinedOutput()
			if len(out) > 0 {
				slog.InfoContext(ctx, "update.sh output", "script", s, "output", string(out))
			}
			if runErr != nil {
				slog.WarnContext(ctx, "update.sh failed", "script", s, "err", runErr)
				return fmt.Errorf("run %s: %w", s, runErr)
			}
			slog.InfoContext(ctx, "update script completed", "script", s)
			return nil
		})
	}
	return g.Wait()
}
