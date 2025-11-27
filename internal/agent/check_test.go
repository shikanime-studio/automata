package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunCheckRetriesUntilSuccess(t *testing.T) {
	dir := t.TempDir()
	flakePath := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(flakePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write flake.nix: %v", err)
	}

	attempts := 0
	oldRunner := runNixFlakeCheck
	defer func() { runNixFlakeCheck = oldRunner }()
	runNixFlakeCheck = func(ctx context.Context, d string) (string, error) {
		attempts++
		if attempts < 3 {
			return "fail", os.ErrInvalid
		}
		return "ok", nil
	}
	oldInitial := runCheckInitialBackoff
	oldMax := runCheckMaxBackoff
	runCheckInitialBackoff = 10 * time.Millisecond
	runCheckMaxBackoff = 20 * time.Millisecond
	defer func() {
		runCheckInitialBackoff = oldInitial
		runCheckMaxBackoff = oldMax
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	start := time.Now()
	err := RunCheck(ctx, dir)
	if err != nil {
		t.Fatalf("RunCheck error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if time.Since(start) < 0 {
		t.Fatalf("invalid duration")
	}
}

func TestRunCheckCancellation(t *testing.T) {
	dir := t.TempDir()
	flakePath := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(flakePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write flake.nix: %v", err)
	}

	oldRunner := runNixFlakeCheck
	defer func() { runNixFlakeCheck = oldRunner }()
	runNixFlakeCheck = func(ctx context.Context, d string) (string, error) {
		return "fail", os.ErrInvalid
	}
	oldInitial := runCheckInitialBackoff
	oldMax := runCheckMaxBackoff
	runCheckInitialBackoff = 10 * time.Millisecond
	runCheckMaxBackoff = 20 * time.Millisecond
	defer func() {
		runCheckInitialBackoff = oldInitial
		runCheckMaxBackoff = oldMax
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := RunCheck(ctx, dir)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
}
