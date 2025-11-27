package utils

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// Major returns the major version of a semver string.
func Major(v string) string {
	return semver.Major(NormalizeSemver(v))
}

// MajorMinor returns the major.minor version of a semver string.
func MajorMinor(v string) string {
	return semver.MajorMinor(NormalizeSemver(v))
}

// PreRelease returns true if the semver string is a prerelease.
func PreRelease(v string) string {
	return semver.Prerelease(NormalizeSemver(v))
}

// Compare compares two semver strings.
func Compare(a, b string) int {
	return semver.Compare(NormalizeSemver(a), NormalizeSemver(b))
}

// NormalizeSemverWithRegex extracts a semver from tag using named capture groups,
// then canonicalizes it by reusing ParseSemver.
func NormalizeSemverWithRegex(re *regexp.Regexp, v string) (string, error) {
	m := re.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("no semver match in tag %q using regex %q", v, re.String())
	}

	raw := getSubexpValue(re, m, "version")
	if raw == "" {
		raw = parseSemverWithRegex(re, m)
	}
	return NormalizeSemver(raw), nil
}

func getSubexpValue(re *regexp.Regexp, m []string, name string) string {
	idx := re.SubexpIndex(name)
	if idx >= 0 && idx < len(m) {
		return m[idx]
	}
	return ""
}

func parseSemverWithRegex(re *regexp.Regexp, m []string) string {
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
	return fmt.Sprintf("v%s", s)
}

// NormalizeSemver normalizes a tag to have a leading 'v' and no 'V' prefix.
func NormalizeSemver(v string) string {
	if strings.HasPrefix(v, "V") {
		return "v" + v[1:]
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
