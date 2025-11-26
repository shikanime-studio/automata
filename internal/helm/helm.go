package helm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shikanime-studio/automata/internal/utils"
)

type ChartRef struct {
	RepoURL string
	Name    string
	Version string
}

type FindLatestOption func(*findLatestOptions)

type findLatestOptions struct {
	exclude           map[string]struct{}
	updateStrategy    utils.StrategyType
	includePreRelease bool
}

func WithExclude(exclude map[string]struct{}) FindLatestOption {
	return func(o *findLatestOptions) {
		o.exclude = exclude
	}
}

func WithStrategyType(strategy utils.StrategyType) FindLatestOption {
	return func(o *findLatestOptions) {
		o.updateStrategy = strategy
	}
}

func WithPreRelease(include bool) FindLatestOption {
	return func(o *findLatestOptions) {
		o.includePreRelease = include
	}
}

func ListVersions(chart *ChartRef) ([]string, error) {
	repoName := fmt.Sprintf("temp-%s-%d", sanitizeRepoName(chart.RepoURL), time.Now().UnixNano())
	add := exec.Command("helm", "repo", "add", repoName, chart.RepoURL, "--force-update")
	add.Env = os.Environ()
	if out, err := add.CombinedOutput(); err != nil {
		slog.Debug("helm repo add failed", "output", string(out), "error", err)
		return nil, err
	}
	upd := exec.Command("helm", "repo", "update")
	upd.Env = os.Environ()
	if out, err := upd.CombinedOutput(); err != nil {
		slog.Debug("helm repo update failed", "output", string(out), "error", err)
		_ = exec.Command("helm", "repo", "remove", repoName).Run()
		return nil, err
	}
	search := exec.Command(
		"helm",
		"search",
		"repo",
		repoName+"/"+chart.Name,
		"--output",
		"json",
		"--versions",
	)
	search.Env = os.Environ()
	out, err := search.Output()
	if err != nil {
		_ = exec.Command("helm", "repo", "remove", repoName).Run()
		return nil, err
	}
	_ = exec.Command("helm", "repo", "remove", repoName).Run()

	var list []map[string]any
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, err
	}
	vers := make([]string, 0, len(list))
	for _, it := range list {
		if v, ok := it["version"].(string); ok && v != "" {
			vers = append(vers, v)
		}
	}
	return vers, nil
}

func FindLatestVersion(chart *ChartRef, opts ...FindLatestOption) (string, error) {
	o := &findLatestOptions{updateStrategy: utils.FullUpdate}
	for _, opt := range opts {
		opt(o)
	}

	baselineSem, err := utils.ParseSemver(chart.Version)
	if err != nil {
		return "", fmt.Errorf("invalid baseline %q: %w", chart.Version, err)
	}

	var baseline string
	switch o.updateStrategy {
	case utils.FullUpdate:
		baseline = baselineSem
	case utils.MinorUpdate:
		baseline = utils.Major(baselineSem)
	case utils.PatchUpdate:
		baseline = utils.MajorMinor(baselineSem)
	default:
		baseline = baselineSem
	}

	vers, err := ListVersions(chart)
	if err != nil {
		return "", err
	}

	bestRaw := ""
	bestSem := ""
	for _, v := range vers {
		var sem string
		sem, err = utils.ParseSemver(v)
		if err != nil {
			slog.Debug("non-semver chart version ignored", "version", v, "err", err)
			continue
		}
		if !o.includePreRelease && utils.PreRelease(sem) != "" {
			slog.Debug("prerelease chart version ignored", "version", v, "sem", sem)
			continue
		}
		if o.exclude != nil {
			if _, ok := o.exclude[v]; ok {
				slog.Debug("chart version excluded", "version", v, "sem", sem)
				continue
			}
		}
		if utils.Compare(sem, baseline) <= 0 {
			continue
		}
		switch o.updateStrategy {
		case utils.MinorUpdate:
			if utils.Major(sem) == baseline {
				if bestSem == "" || utils.Compare(sem, bestSem) > 0 {
					bestSem = sem
					bestRaw = v
				}
			} else {
				slog.Debug("chart version excluded by strategy", "version", v, "sem", sem, "baseline", baseline)
			}
		case utils.PatchUpdate:
			if utils.MajorMinor(sem) == baseline {
				if bestSem == "" || utils.Compare(sem, bestSem) > 0 {
					bestSem = sem
					bestRaw = v
				}
			} else {
				slog.Debug("chart version excluded by strategy", "version", v, "sem", sem, "baseline", baseline)
			}
		default:
			if bestSem == "" || utils.Compare(sem, bestSem) > 0 {
				bestSem = sem
				bestRaw = v
			}
		}
	}

	return bestRaw, nil
}

func sanitizeRepoName(u string) string {
	b := strings.Builder{}
	for _, r := range u {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}
