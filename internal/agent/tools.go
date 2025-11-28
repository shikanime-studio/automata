package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/shikanime-studio/automata/internal/fsutil"
)

// Seek text tool combines reading and searching capabilities.
type seekArgs struct {
	Root           string
	Pattern        string
	Glob           string
	IncludeContent bool
	MaxFiles       int
}
type seekHit struct {
	Line int
	Text string
}
type seekFile struct {
	Path    string
	Hits    []seekHit
	Content string
}
type seekResult struct{ Files []seekFile }

// NewSeekTextTool returns a tool that searches for a regex in files and, when requested,
// includes the full content of matched files. It honors .gitignore and supports glob filters.
func NewSeekTextTool() (tool.Tool, error) {
	return functiontool.New[seekArgs, seekResult](functiontool.Config{
		Name:        "seek_text",
		Description: "Search files with regex (honors .gitignore) and optionally include matched file content",
	}, func(tc tool.Context, in seekArgs) (seekResult, error) {
		re, err := regexp.Compile(in.Pattern)
		if err != nil {
			return seekResult{}, err
		}
		var files []seekFile
		count := 0
		err = fsutil.WalkDirWithGitignore(tc, in.Root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if in.Glob != "" {
				matched, mErr := path.Match(in.Glob, filepath.Base(p))
				if mErr != nil {
					return mErr
				}
				if !matched {
					return nil
				}
			}
			f, oErr := os.Open(p)
			if oErr != nil {
				return oErr
			}
			defer func() { _ = f.Close() }()
			sc := bufio.NewScanner(f)
			ln := 0
			var hits []seekHit
			for sc.Scan() {
				ln++
				t := sc.Text()
				if re.MatchString(t) {
					hits = append(hits, seekHit{Line: ln, Text: t})
				}
			}
			if err := sc.Err(); err != nil {
				return err
			}
			if len(hits) > 0 {
				sf := seekFile{Path: p, Hits: hits}
				if in.IncludeContent {
					b, rErr := os.ReadFile(p)
					if rErr != nil {
						return rErr
					}
					sf.Content = string(b)
				}
				files = append(files, sf)
				count++
				if in.MaxFiles > 0 && count >= in.MaxFiles {
					return fmt.Errorf("seek limit reached")
				}
			}
			return nil
		})
		if err != nil && err.Error() != "seek limit reached" {
			return seekResult{}, err
		}
		return seekResult{Files: files}, nil
	})
}

type listArgs struct {
	Root        string
	Glob        string
	Regex       string
	Recurse     bool
	IncludeDirs bool
}
type listEntry struct {
	Path  string
	IsDir bool
}
type listResult struct{ Entries []listEntry }

// NewListDirTool returns a tool that lists directory entries with optional recursion,
// glob and regex filters, honoring .gitignore when recursing.
func NewListDirTool() (tool.Tool, error) {
	return functiontool.New[listArgs, listResult](functiontool.Config{
		Name:        "list_dir",
		Description: "List directory entries, with optional recursion, glob and regex filters (honors .gitignore)",
	}, func(tc tool.Context, in listArgs) (listResult, error) {
		var entries []listEntry
		match := func(name string) (bool, error) {
			if in.Glob != "" {
				ok, err := path.Match(in.Glob, name)
				if err != nil {
					return false, err
				}
				if !ok {
					return false, nil
				}
			}
			if in.Regex != "" {
				re, err := regexp.Compile(in.Regex)
				if err != nil {
					return false, err
				}
				if !re.MatchString(name) {
					return false, nil
				}
			}
			return true, nil
		}
		if !in.Recurse {
			items, err := os.ReadDir(in.Root)
			if err != nil {
				return listResult{}, err
			}
			for _, d := range items {
				ok, err := match(d.Name())
				if err != nil {
					return listResult{}, err
				}
				if !ok {
					continue
				}
				if d.IsDir() && !in.IncludeDirs {
					continue
				}
				entries = append(entries, listEntry{Path: filepath.Join(in.Root, d.Name()), IsDir: d.IsDir()})
			}
			return listResult{Entries: entries}, nil
		}
		err := fsutil.WalkDirWithGitignore(tc, in.Root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			ok, mErr := match(name)
			if mErr != nil {
				return mErr
			}
			if !ok {
				return nil
			}
			if d.IsDir() && !in.IncludeDirs {
				return nil
			}
			entries = append(entries, listEntry{Path: p, IsDir: d.IsDir()})
			return nil
		})
		if err != nil {
			return listResult{}, err
		}
		return listResult{Entries: entries}, nil
	})
}

type textPatchArgs struct {
	Root  string
	Patch string
	Strip int
}
type textPatchResult struct{ Output string }

// NewApplyTextPatchTool returns a tool that applies a unified diff patch using git apply.
func NewApplyTextPatchTool() (tool.Tool, error) {
	return functiontool.New[textPatchArgs, textPatchResult](functiontool.Config{
		Name:        "apply_text_patch",
		Description: "Apply a unified diff patch via git apply",
	}, func(tc tool.Context, in textPatchArgs) (textPatchResult, error) {
		if in.Strip < 0 {
			in.Strip = 0
		}
		tmp, err := os.CreateTemp("", "patch-*.diff")
		if err != nil {
			return textPatchResult{}, err
		}
		defer func() { _ = os.Remove(tmp.Name()) }()
		if _, err := tmp.WriteString(in.Patch); err != nil {
			_ = tmp.Close()
			return textPatchResult{}, err
		}
		_ = tmp.Close()
		args := []string{"apply", "--unsafe-paths", "--reject", "--whitespace=nowarn"}
		if in.Strip > 0 {
			args = append(args, "-p", strconv.Itoa(in.Strip))
		}
		args = append(args, tmp.Name())
		cmd := exec.CommandContext(tc, "git", args...)
		if in.Root != "" {
			cmd.Dir = in.Root
		}
		cmd.Env = os.Environ()
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return textPatchResult{Output: string(out)}, fmt.Errorf("git apply failed: %w", runErr)
		}
		return textPatchResult{Output: string(out)}, nil
	})
}
