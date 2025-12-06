// Package fsutil provides filesystem utility functions.
package fsutil

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
)

// SkipHidden returns a WalkDirFunc that skips hidden directories.
// It does not skip the root directory itself.
func SkipHidden(root string, next fs.WalkDirFunc) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories, but not the root itself
		if d.IsDir() && IsHidden(path) && path != root {
			return filepath.SkipDir
		}
		return next(path, d, err)
	}
}

// IsHidden reports whether the given path is hidden.
func IsHidden(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}

// SkipGitIgnored returns a WalkDirFunc that skips files ignored by git.
// It requires the root directory to run the git command in.
func SkipGitIgnored(ctx context.Context, root string, next fs.WalkDirFunc) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if IsGitIgnored(ctx, root, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return next(path, d, err)
	}
}

// IsGitIgnored reports whether the given path is ignored by git.
func IsGitIgnored(ctx context.Context, root, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "check-ignore", "-q", "--", path)
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}
