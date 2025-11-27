package updater

import (
	"regexp"
	"testing"
)

func TestStrategy_DetectsTypes(t *testing.T) {
	cases := map[string]StrategyType{
		"v1":        MajorUpdate,
		"1":         MajorUpdate,
		"v1.1":      MajorMinorUpdate,
		"1.1":       MajorMinorUpdate,
		"v1.1.1":    CanonicalUpdate,
		"1.1.1":     CanonicalUpdate,
		"v2.0.0":    CanonicalUpdate,
		"v2.1.0":    CanonicalUpdate,
		"v2.1.0-rc": PreReleaseUpdate,
		"v2.1.0+b1": PreReleaseUpdate,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Strategy(v)
			if err != nil {
				t.Fatalf("Strategy(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Strategy(%q)=%v want %v", v, got, want)
			}
		})
	}
}

func TestStrategy_WithTransform(t *testing.T) {
	re := regexp.MustCompile(`^release-(?P<major>\d+)(?:\.(?P<minor>\d+))?(?:\.(?P<patch>\d+))?$`)
	cases := map[string]StrategyType{
		"release-1":     MajorUpdate,
		"release-1.1":   MajorMinorUpdate,
		"release-1.1.1": CanonicalUpdate,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Strategy(v, WithTransform(re))
			if err != nil {
				t.Fatalf("Strategy(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Strategy(%q)=%v want %v", v, got, want)
			}
		})
	}
}

func TestStrategy_WithVersionCapture(t *testing.T) {
	re := regexp.MustCompile(`^tag-(?P<version>v\d+(?:\.\d+){0,2}(?:-[^+]+)?(?:\+.+)?)$`)
	cases := map[string]StrategyType{
		"tag-v1":         MajorUpdate,
		"tag-v1.1":       MajorMinorUpdate,
		"tag-v1.1.1":     CanonicalUpdate,
		"tag-v1.2.3-rc1": PreReleaseUpdate,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			got, err := Strategy(v, WithTransform(re))
			if err != nil {
				t.Fatalf("Strategy(%q) error: %v", v, err)
			}
			if got != want {
				t.Fatalf("Strategy(%q)=%v want %v", v, got, want)
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
				t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
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
				t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
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
				t.Fatalf("Compare(%q,%q) error: %v", c.baseline, c.target, err)
			}
			if got != c.want {
				t.Fatalf("Compare(%q,%q)=%v want %v", c.baseline, c.target, got, c.want)
			}
		})
	}
}
