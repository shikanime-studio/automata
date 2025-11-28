package agent

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/shikanime-studio/automata/internal/fsutil"
)

type (
	readArgs   struct{ Path string }
	readResult struct{ Content string }
)

// NewReadFileTool returns a tool that reads a file and returns its content.
func NewReadFileTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Read a file and return its content",
	}, func(_ tool.Context, in readArgs) (readResult, error) {
		b, err := os.ReadFile(in.Path)
		if err != nil {
			return readResult{}, err
		}
		return readResult{Content: string(b)}, nil
	})
}

type writeArgs struct {
	Path    string
	Content string
}
type writeResult struct{ Bytes int }

// NewWriteFileTool returns a tool that writes content to a file.
func NewWriteFileTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "write_file",
		Description: "Write content to a file, creating it if needed",
	}, func(_ tool.Context, in writeArgs) (writeResult, error) {
		if err := os.MkdirAll(filepath.Dir(in.Path), 0o755); err != nil {
			return writeResult{}, err
		}
		if err := os.WriteFile(in.Path, []byte(in.Content), 0o644); err != nil {
			return writeResult{}, err
		}
		return writeResult{Bytes: len(in.Content)}, nil
	})
}

type replaceArgs struct {
	Path    string
	Pattern string
	Replace string
}
type replaceResult struct{ Count int }

// NewReplaceTextTool returns a tool that performs regex replacements in a file.
func NewReplaceTextTool() (tool.Tool, error) {
	return functiontool.New[replaceArgs, replaceResult](functiontool.Config{
		Name:        "replace_text",
		Description: "Regex replace occurrences in a file",
	}, func(_ tool.Context, in replaceArgs) (replaceResult, error) {
		re, err := regexp.Compile(in.Pattern)
		if err != nil {
			return replaceResult{}, err
		}
		b, err := os.ReadFile(in.Path)
		if err != nil {
			return replaceResult{}, err
		}
		s := string(b)
		idx := re.FindAllStringIndex(s, -1)
		out := re.ReplaceAllString(s, in.Replace)
		if err := os.WriteFile(in.Path, []byte(out), 0o644); err != nil {
			return replaceResult{}, err
		}
		return replaceResult{Count: len(idx)}, nil
	})
}

type insertArgs struct {
	Path string
	Line int
	Text string
}
type insertResult struct{ Lines int }

// NewInsertTextTool returns a tool that inserts text at a given line number.
func NewInsertTextTool() (tool.Tool, error) {
	return functiontool.New[insertArgs, insertResult](functiontool.Config{
		Name:        "insert_text",
		Description: "Insert text at a 1-based line number",
	}, func(_ tool.Context, in insertArgs) (insertResult, error) {
		f, err := os.Open(in.Path)
		if err != nil {
			return insertResult{}, err
		}
		defer func() { _ = f.Close() }()
		var lines []string
		s := bufio.NewScanner(f)
		for s.Scan() {
			lines = append(lines, s.Text())
		}
		if err := s.Err(); err != nil {
			return insertResult{}, err
		}
		if in.Line < 1 {
			in.Line = 1
		}
		if in.Line > len(lines)+1 {
			in.Line = len(lines) + 1
		}
		i := in.Line - 1
		lines = append(lines[:i], append([]string{in.Text}, lines[i:]...)...)
		out := strings.Join(lines, "\n")
		if err := os.WriteFile(in.Path, []byte(out), 0o644); err != nil {
			return insertResult{}, err
		}
		return insertResult{Lines: len(lines)}, nil
	})
}

type deleteArgs struct {
	Path  string
	Start int
	End   int
}
type deleteResult struct{ Lines int }

// NewDeleteLinesTool returns a tool that deletes an inclusive line range.
func NewDeleteLinesTool() (tool.Tool, error) {
	return functiontool.New[deleteArgs, deleteResult](functiontool.Config{
		Name:        "delete_lines",
		Description: "Delete lines [start,end] inclusive (1-based)",
	}, func(_ tool.Context, in deleteArgs) (deleteResult, error) {
		f, err := os.Open(in.Path)
		if err != nil {
			return deleteResult{}, err
		}
		defer func() { _ = f.Close() }()
		var lines []string
		s := bufio.NewScanner(f)
		for s.Scan() {
			lines = append(lines, s.Text())
		}
		if err := s.Err(); err != nil {
			return deleteResult{}, err
		}
		if in.Start < 1 {
			in.Start = 1
		}
		if in.End < in.Start {
			in.End = in.Start
		}
		if in.Start > len(lines) {
			return deleteResult{Lines: len(lines)}, nil
		}
		if in.End > len(lines) {
			in.End = len(lines)
		}
		i := in.Start - 1
		j := in.End
		lines = append(lines[:i], lines[j:]...)
		out := strings.Join(lines, "\n")
		if err := os.WriteFile(in.Path, []byte(out), 0o644); err != nil {
			return deleteResult{}, err
		}
		return deleteResult{Lines: len(lines)}, nil
	})
}

type searchArgs struct {
	Root    string
	Pattern string
	Glob    string
}
type searchHit struct {
	Path string
	Line int
	Text string
}
type searchResult struct{ Hits []searchHit }

// NewSearchTextTool returns a tool that searches text using regex.
func NewSearchTextTool() (tool.Tool, error) {
	return functiontool.New[searchArgs, searchResult](functiontool.Config{
		Name:        "search_text",
		Description: "Search text with regex over files honoring .gitignore",
	}, func(_ tool.Context, in searchArgs) (searchResult, error) {
		re, err := regexp.Compile(in.Pattern)
		if err != nil {
			return searchResult{}, err
		}
		var hits []searchHit
		err = fsutil.WalkDirWithGitignore(in.Root, func(p string, d os.DirEntry, err error) error {
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
			for sc.Scan() {
				ln++
				t := sc.Text()
				if re.MatchString(t) {
					hits = append(hits, searchHit{Path: p, Line: ln, Text: t})
				}
			}
			return sc.Err()
		})
		if err != nil {
			return searchResult{}, err
		}
		return searchResult{Hits: hits}, nil
	})
}
