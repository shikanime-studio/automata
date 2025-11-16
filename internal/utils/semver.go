package utils

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// ParseSemver parses a tag into canonical semver (with leading 'v') or returns an error.
func ParseSemver(v string) (string, error) {
	canon := semver.Canonical(NormalizeSemverPrefix(v))
	if canon == "" {
		return "", fmt.Errorf("invalid semver %q", v)
	}
	return canon, nil
}

// Major returns the major version of a semver string.
func Major(v string) string {
	return semver.Major(NormalizeSemverPrefix(v))
}

// MajorMinor returns the major.minor version of a semver string.
func MajorMinor(v string) string {
	return semver.MajorMinor(NormalizeSemverPrefix(v))
}

// PreRelease returns true if the semver string is a prerelease.
func PreRelease(v string) string {
	return semver.Prerelease(NormalizeSemverPrefix(v))
}

// Compare compares two semver strings.
func Compare(a, b string) int {
	return semver.Compare(NormalizeSemverPrefix(a), NormalizeSemverPrefix(b))
}

// ParseSemverWithRegex extracts a semver from tag using named capture groups,
// then canonicalizes it by reusing ParseSemver.
func ParseSemverWithRegex(re *regexp.Regexp, v string) (string, error) {
	m := re.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("no semver match in tag %q", v)
	}

	raw := getSubexpValue(re, m, "version")
	if raw != "" {
		canon, err := ParseSemver(raw)
		if err == nil {
			return canon, nil
		}
	}

	built, ok := parseSemverWithRegex(re, m)
	if !ok {
		return "", fmt.Errorf("no version groups matched in tag %q", v)
	}
	canon, err := ParseSemver(built)
	if err != nil {
		return "", err
	}
	return canon, nil
}

func getSubexpValue(re *regexp.Regexp, m []string, name string) string {
	idx := re.SubexpIndex(name)
	if idx >= 0 && idx < len(m) {
		return m[idx]
	}
	return ""
}

func parseSemverWithRegex(re *regexp.Regexp, m []string) (string, bool) {
	maj := getSubexpValue(re, m, "major")
	if maj == "" {
		maj = "0"
	}
	min := getSubexpValue(re, m, "minor")
	if min == "" {
		min = "0"
	}
	pat := getSubexpValue(re, m, "patch")
	if pat == "" {
		pat = "0"
	}
	pre := getSubexpValue(re, m, "prerelease")
	bld := getSubexpValue(re, m, "build")
	s := maj + "." + min + "." + pat
	if pre != "" {
		s += "-" + pre
	}
	if bld != "" {
		s += "+" + bld
	}
	return s, true
}

// NormalizeSemverPrefix normalizes a tag to have a leading 'v' and no 'V' prefix.
func NormalizeSemverPrefix(v string) string {
	if strings.HasPrefix(v, "V") {
		return "v" + v[1:]
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
