package app

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/utils"
	"github.com/spf13/cobra"
)

// NewUpdateFlakeCmd runs `nix flake update` for directories containing flake.nix.
func NewUpdateFlakeCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "flake DIR...",
        Short: "Run nix flake update where flake.nix exists",
        Args:  cobra.MinimumNArgs(1),
        RunE: func(_ *cobra.Command, args []string) error {
            for _, a := range args {
                root := strings.TrimSpace(a)
                if root == "" {
                    continue
                }
                if err := runUpdateFlake(root); err != nil {
                    return err
                }
            }
            return nil
        },
    }
}

// runUpdateFlake walks the directory tree and executes `nix flake update` for each found flake.nix.
func runUpdateFlake(root string) error {
	var flakeDirs []string
	err := utils.WalkDirWithGitignore(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "flake.nix" {
			flakeDirs = append(flakeDirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan for flake.nix: %w", err)
	}

	if len(flakeDirs) == 0 {
		slog.Info("no flake.nix files found", "root", root)
		return nil
	}

	for _, dir := range flakeDirs {
		slog.Info("running nix flake update", "dir", dir)
		cmd := exec.Command("nix", "flake", "update")
		cmd.Dir = dir
		cmd.Env = os.Environ()

		out, err := cmd.CombinedOutput()
		if len(out) > 0 {
			slog.Info("nix flake update output", "dir", dir, "output", string(out))
		}
		if err != nil {
			slog.Warn("nix flake update failed", "dir", dir, "err", err)
			return fmt.Errorf("nix flake update in %s: %w", dir, err)
		}
		slog.Info("nix flake update completed", "dir", dir)
	}

	return nil
}
