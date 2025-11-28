// Package updater provides semver utilities and an update interface.
package updater

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// StrategyType describes the kind of version update strategy.
type StrategyType int

// StrategyType values for selecting update behavior.
const (
	CanonicalUpdate StrategyType = iota
	MajorMinorUpdate
	MajorUpdate
	PreReleaseUpdate
)

type options struct {
	transformRegex *regexp.Regexp
}

// Option configures semver parsing and comparison behavior.
type Option = func(*options)

// WithTransform uses a regex with named groups to extract semver parts.
func WithTransform(re *regexp.Regexp) Option {
	return func(o *options) {
		o.transformRegex = re
	}
}

func makeOptions(opts ...Option) options {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Comparison indicates relative ordering between two versions.
type Comparison int

// Comparison values for ordering semantics.
const (
	Equal Comparison = iota
	Greater
	Less
)

// Compare compares two versions using consistent strategy and canonicalization.
func Compare(baseline, target string, opts ...Option) (Comparison, error) {
	if baseline == "latest" {
		return Greater, nil
	}

	baselineStrategy, err := Strategy(baseline, opts...)
	if err != nil {
		return 0, fmt.Errorf("failed to determine strategy for baseline %q: %w", baseline, err)
	}
	targetStrategy, err := Strategy(target, opts...)
	if err != nil {
		slog.Warn("failed to determine strategy for target %q: %v", target, err)
		return Less, nil
	}
	if targetStrategy != baselineStrategy {
		return Less, nil
	}

	baseline, err = Canonical(baseline, opts...)
	if err != nil {
		return Less, fmt.Errorf("failed to canonicalize baseline %q: %w", baseline, err)
	}
	target, err = Canonical(target, opts...)
	if err != nil {
		return Less, fmt.Errorf("failed to canonicalize target %q: %w", target, err)
	}

	switch cmp := semver.Compare(baseline, target); {
	case cmp == 0:
		return Equal, nil
	case cmp < 0:
		return Greater, nil
	default:
		return Less, nil
	}
}

// Strategy determines the update strategy for a version string.
func Strategy(v string, opts ...Option) (StrategyType, error) {
	o := makeOptions(opts...)

	if o.transformRegex != nil {
		m := o.transformRegex.FindStringSubmatch(v)
		if m == nil {
			return 0, fmt.Errorf(
				"no semver match in tag %q using regex %q",
				v,
				o.transformRegex.String(),
			)
		}

		v = getSubexpValue(o.transformRegex, m, "version")
		if v == "" {
			if getSubexpValue(o.transformRegex, m, "prerelease") != "" {
				return PreReleaseUpdate, nil
			}
			if getSubexpValue(o.transformRegex, m, "patch") != "" {
				return CanonicalUpdate, nil
			}
			if getSubexpValue(o.transformRegex, m, "minor") != "" {
				return MajorMinorUpdate, nil
			}
			if getSubexpValue(o.transformRegex, m, "major") != "" {
				return MajorUpdate, nil
			}
			return 0, fmt.Errorf("unable to determine strategy from tag %q", v)
		}
	}

	v, err := Canonical(v)
	if err != nil {
		return 0, err
	}

	switch {
	case semver.Prerelease(v) != "" || semver.Build(v) != "":
		return PreReleaseUpdate, nil
	case v == semver.Canonical(v):
		return CanonicalUpdate, nil
	case v == semver.MajorMinor(v):
		return MajorMinorUpdate, nil
	case v == semver.Major(v):
		return MajorUpdate, nil
	default:
		return 0, fmt.Errorf("unable to determine strategy from tag %q", v)
	}
}

// Canonical normalizes a tag to have a leading 'v' and no 'V' prefix.
func Canonical(v string, opts ...Option) (string, error) {
	o := makeOptions(opts...)

	if o.transformRegex != nil {
		m := o.transformRegex.FindStringSubmatch(v)
		if m == nil {
			return "", fmt.Errorf(
				"no semver match in tag %q using regex %q",
				v,
				o.transformRegex.String(),
			)
		}

		v = getSubexpValue(o.transformRegex, m, "version")
		if v == "" {
			v = canonicalWithRegex(o.transformRegex, m)
		}
	}

	if strings.HasPrefix(v, "V") {
		return "v" + v[1:], nil
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v, nil
	}

	return v, nil
}

func getSubexpValue(re *regexp.Regexp, m []string, name string) string {
	idx := re.SubexpIndex(name)
	if idx >= 0 && idx < len(m) {
		return m[idx]
	}
	return ""
}

func canonicalWithRegex(re *regexp.Regexp, m []string) string {
	maj := getSubexpValue(re, m, "major")
	if maj == "" {
		maj = "0"
	}
	minor := getSubexpValue(re, m, "minor")
	if minor == "" {
		minor = "0"
	}
	pat := getSubexpValue(re, m, "patch")
	if pat == "" {
		pat = "0"
	}
	pre := getSubexpValue(re, m, "prerelease")
	bld := getSubexpValue(re, m, "build")
	s := maj + "." + minor + "." + pat
	if pre != "" {
		s += "-" + pre
	}
	if bld != "" {
		s += "+" + bld
	}
	return fmt.Sprintf("v%s", s)
}
