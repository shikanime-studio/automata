package updater

import (
	"regexp"
	"testing"
)

func TestType_DetectsTypes(t *testing.T) {
	cases := map[string]VersionType{
		"v1":        MajorVersion,
		"1":         MajorVersion,
		"v1.1":      MajorMinorVersion,
		"1.1":       MajorMinorVersion,
		"v1.1.1":    CanonicalVersion,
		"1.1.1":     CanonicalVersion,
		"v2.0.0":    CanonicalVersion,
		"v2.1.0":    CanonicalVersion,
		"v2.1.0-rc": PreReleaseVersion,
		"v2.1.0+b1": PreReleaseVersion,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Type(v)
			if err != nil {
				t.Fatalf("Type(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Type(%q)=%v want %v", v, got, want)
			}
		})
	}
}

func TestType_WithTransform(t *testing.T) {
	re := regexp.MustCompile(`^release-(?P<major>\d+)(?:\.(?P<minor>\d+))?(?:\.(?P<patch>\d+))?$`)
	cases := map[string]VersionType{
		"release-1":     MajorVersion,
		"release-1.1":   MajorMinorVersion,
		"release-1.1.1": CanonicalVersion,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Type(v, WithTransform(re))
			if err != nil {
				t.Fatalf("Type(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Type(%q)=%v want %v", v, got, want)
			}
		})
	}
}

func TestType_WithVersionCapture(t *testing.T) {
	re := regexp.MustCompile(`^tag-(?P<version>v\d+(?:\.\d+){0,2}(?:-[^+]+)?(?:\+.+)?)$`)
	cases := map[string]VersionType{
		"tag-v1":         MajorVersion,
		"tag-v1.1":       MajorMinorVersion,
		"tag-v1.1.1":     CanonicalVersion,
		"tag-v1.2.3-rc1": PreReleaseVersion,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Type(v, WithTransform(re))
			if err != nil {
				t.Fatalf("Type(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Type(%q)=%v want %v", v, got, want)
			}
		})
	}
}

func TestCompare_MajorUpdate(t *testing.T) {
	cases := []struct {
		baseline string
		target   string
		want     Comparison
	}{
		{"v1", "v2", Greater},
		{"1", "2", Greater},
		{"v1", "v1", Equal},
		{"v2", "v1", Less},
		{"v1", "v1.1", Less},
	}
	for _, c := range cases {
		t.Run(c.baseline+"->"+c.target, func(t *testing.T) {
			got, err := Compare(c.baseline, c.target)
			if err != nil {
				if !IsNotValid(err) {
					t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
				}
				return
			}
			if got != c.want {
				t.Fatalf("Compare(%q,%q)=%v want %v", c.baseline, c.target, got, c.want)
			}
		})
	}
}

func TestCompare_MajorMinorUpdate(t *testing.T) {
	cases := []struct {
		baseline string
		target   string
		want     Comparison
	}{
		{"v1.1", "v1.2", Greater},
		{"1.1", "1.1", Equal},
		{"v1.2", "v1.1", Less},
		{"v1.1", "v2", Less},
		{"v1.1", "v1.1.1", Less},
	}
	for _, c := range cases {
		t.Run(c.baseline+"->"+c.target, func(t *testing.T) {
			got, err := Compare(c.baseline, c.target)
			if err != nil {
				if !IsNotValid(err) {
					t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
				}
				return
			}
			if got != c.want {
				t.Fatalf("Compare(%q,%q)=%v want %v", c.baseline, c.target, got, c.want)
			}
		})
	}
}

func TestCompare_CanonicalUpdate(t *testing.T) {
	cases := []struct {
		baseline string
		target   string
		want     Comparison
	}{
		{"v1.1.1", "v1.1.2", Greater},
		{"1.1.1", "1.1.1", Equal},
		{"v1.1.2", "v1.1.1", Less},
		{"v1.1.1", "v1.2", Less},
		{"v1.1.1", "v2", Less},
	}
	for _, c := range cases {
		t.Run(c.baseline+"->"+c.target, func(t *testing.T) {
			got, err := Compare(c.baseline, c.target)
			if err != nil {
				if !IsNotValid(err) {
					t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
				}
				return
			}
			if got != c.want {
				t.Fatalf("Compare(%q,%q)=%v want %v", c.baseline, c.target, got, c.want)
			}
		})
	}
}

func TestCompare_TargetError(t *testing.T) {
	re := regexp.MustCompile(`^.*(?P<version>v\d+(?:\.\d+){0,2}(?:-[^+]+)?(?:\+.+)?)$`)
	_, err := Compare("v1.1.1", "1.1.2", WithTransform(re))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !IsNotValid(err) {
		t.Fatalf("expected invalid target error, got %v", err)
	}
}

func TestPolicy(t *testing.T) {
	cases := map[string]PolicyType{
		"v0.0.1": PathRelease,
		"0.0.5":  PathRelease,
		"v0.1":   MinorRelease,
		"0.2.0":  MinorRelease,
		"v1":     MajorRelease,
		"1.0.0":  MajorRelease,
		"v2.3.4": MajorRelease,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Policy(v)
			if err != nil {
				t.Fatalf("Policy(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Policy(%q)=%v want %v", v, got, want)
			}
		})
	}
}
