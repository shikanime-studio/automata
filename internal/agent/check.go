package agent

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
)

// backoff removed

func RunCheck(ctx context.Context, root string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	slog.Info("running nix flake check", "dir", root)
	out, runErr := runNixFlakeCheck(ctx, root)
	if len(out) > 0 {
		slog.Info("nix flake check output", "dir", root, "output", string(out))
	}
	if runErr == nil {
		slog.Info("nix flake check succeeded", "dir", root)
		return nil
	}
	slog.Warn("nix flake check failed", "dir", root, "err", runErr)
	return runErr
}

var runNixFlakeCheck = func(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "nix", "flake", "check", "--no-pure-eval")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}
