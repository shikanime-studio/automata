// Package github provides helpers for GitHub Actions references and API.
package github

import (
	"fmt"
	"strings"
)

// ActionRef represents a parsed GitHub Action reference "owner/repo@version".
type ActionRef struct {
	Owner   string
	Repo    string
	Version string
}

// String returns the canonical "owner/repo@version" form of the action
// reference.
func (a ActionRef) String() string {
	return fmt.Sprintf("%s/%s@%s", a.Owner, a.Repo, a.Version)
}

// ParseActionRef parses a GitHub Actions `uses` string like "owner/repo@v1".
func ParseActionRef(uses string) (ref *ActionRef, err error) {
	s := strings.TrimSpace(uses)
	if s == "" {
		return nil, fmt.Errorf("empty uses")
	}
	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid uses: missing '@'")
	}
	path := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])
	if path == "" || version == "" {
		return nil, fmt.Errorf("invalid uses: empty action or version")
	}
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 2 || strings.TrimSpace(pathParts[0]) == "" ||
		strings.TrimSpace(pathParts[1]) == "" {
		return nil, fmt.Errorf("invalid action path %q, expected <owner>/<repo>", path)
	}
	return &ActionRef{Owner: pathParts[0], Repo: pathParts[1], Version: version}, nil
}
