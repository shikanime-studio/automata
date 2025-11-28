// Package updater provides semver utilities and an update interface.
package updater

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

var (
	ErrPolicyRejection = errors.New("policy rejection")
	ErrTypeMismatch    = errors.New("type mismatch")
	ErrInvalidTarget   = errors.New("invalid target version")
)

func IsNotValid(err error) bool {
	return errors.Is(err, ErrInvalidTarget) || errors.Is(err, ErrPolicyRejection) || errors.Is(err, ErrTypeMismatch)
}

type options struct {
	transformRegex *regexp.Regexp
	policy         *PolicyType
}

// Option configures semver parsing and comparison behavior.
type Option = func(*options)

// WithTransform uses a regex with named groups to extract semver parts.
func WithTransform(re *regexp.Regexp) Option {
	return func(o *options) {
		o.transformRegex = re
	}
}

func WithPolicy(ut PolicyType) Option {
	return func(o *options) {
		o.policy = &ut
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

	baselineType, err := Type(baseline, opts...)
	if err != nil {
		return Equal, fmt.Errorf("failed to determine policy for baseline %q: %w", baseline, err)
	}
	targetType, err := Type(target, opts...)
	if err != nil {
		return Equal, fmt.Errorf("%w: %v", ErrInvalidTarget, err)
	}
	if targetType != baselineType {
		return Equal, fmt.Errorf("%w: type mismatch: %v != %v", ErrTypeMismatch, targetType, baselineType)
	}

	baseline, err = Canonical(baseline, opts...)
	if err != nil {
		return Equal, fmt.Errorf("failed to canonicalize baseline %q: %w", baseline, err)
	}
	target, err = Canonical(target, opts...)
	if err != nil {
		return Equal, fmt.Errorf("%w: %v", ErrInvalidTarget, err)
	}

	switch cmp := semver.Compare(baseline, target); {
	case cmp == 0:
		return Equal, nil
	case cmp < 0:
		o := makeOptions(opts...)
		if o.policy != nil {
			pol, err := Policy(baseline)
			if err != nil {
				return Equal, fmt.Errorf("%w: %v", ErrPolicyRejection, err)
			}
			if *o.policy != pol {
				return Equal, fmt.Errorf("%w: policy mismatch: %v != %v", ErrPolicyRejection, pol, *o.policy)
			}
		}
		return Greater, nil
	default:
		return Less, nil
	}
}

// VersionType describes the kind of version update strategy.
type VersionType int

// TypeType values for selecting update behavior.
const (
	CanonicalVersion VersionType = iota
	MajorMinorVersion
	MajorVersion
	PreReleaseVersion
)

// Type determines the update strategy for a version string.
func Type(v string, opts ...Option) (VersionType, error) {
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
				return PreReleaseVersion, nil
			}
			if getSubexpValue(o.transformRegex, m, "patch") != "" {
				return CanonicalVersion, nil
			}
			if getSubexpValue(o.transformRegex, m, "minor") != "" {
				return MajorMinorVersion, nil
			}
			if getSubexpValue(o.transformRegex, m, "major") != "" {
				return MajorVersion, nil
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
		return PreReleaseVersion, nil
	case v == semver.Major(v):
		return MajorVersion, nil
	case v == semver.MajorMinor(v):
		return MajorMinorVersion, nil
	case v == semver.Canonical(v):
		return CanonicalVersion, nil
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

type PolicyType int

const (
	MajorRelease PolicyType = iota
	PathRelease
	MinorRelease
)

func Policy(baseline string) (PolicyType, error) {
	major, minor, patch, err := semverParts(baseline)
	if err != nil {
		return MajorRelease, err
	}
	if major == 0 && minor == 0 && patch > 0 {
		return PathRelease, nil
	}
	if major == 0 && minor > 0 {
		return MinorRelease, nil
	}
	return MajorRelease, nil
}

func semverParts(v string) (int, int, int, error) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	switch len(parts) {
	case 1:
		parts = []string{parts[0], "0", "0"}
	case 2:
		parts = []string{parts[0], parts[1], "0"}
	default:
		parts = parts[:3]
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}
	pat, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}
	return maj, min, pat, nil
}
