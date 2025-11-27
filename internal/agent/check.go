package agent

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

var runCheckInitialBackoff = 1 * time.Second
var runCheckMaxBackoff = 1 * time.Minute

func RunCheck(ctx context.Context, root string) error {
	backoff := runCheckInitialBackoff
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		slog.InfoContext(ctx, "running nix flake check", "dir", root)
		out, runErr := runNixFlakeCheck(ctx, root)
		if len(out) > 0 {
			slog.InfoContext(ctx, "nix flake check output", "dir", root, "output", string(out))
		}
		if runErr == nil {
			slog.InfoContext(ctx, "nix flake check succeeded", "dir", root)
			return nil
		}
		slog.WarnContext(ctx, "nix flake check failed", "dir", root, "err", runErr)

		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		if backoff < runCheckMaxBackoff {
			backoff *= 2
			if backoff > runCheckMaxBackoff {
				backoff = runCheckMaxBackoff
			}
		}
	}
}

var runNixFlakeCheck = func(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "nix", "flake", "check", "--no-pure-eval")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}
