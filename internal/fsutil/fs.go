// Package fsutil provides filesystem helpers integrated with gitignore behavior.
package fsutil

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
)

// IsGitIgnored reports whether the given path is ignored by git relative to
// the repository root.
func IsGitIgnored(ctx context.Context, root, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "check-ignore", "-q", "--", path)
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

// WalkDirWithGitignore walks `root` like `filepath.WalkDir`, skipping paths
// ignored by git (`git check-ignore`). Ignored directories are not descended.
func WalkDirWithGitignore(ctx context.Context, root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if IsGitIgnored(ctx, root, path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return fn(path, d, err)
	})
}
