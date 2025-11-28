package kio

import (
	"context"
	"testing"

	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/shikanime-studio/automata/internal/helm"
	update "github.com/shikanime-studio/automata/internal/updater"
)

type fakeHelmUpdater struct {
	latest string
	err    error
}

func (f fakeHelmUpdater) Update(
	_ context.Context,
	_ *helm.ChartRef,
	_ ...update.Option,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.latest, nil
}

func TestUpdateK0sctlConfigchart_UpdatesVersion(t *testing.T) {
	doc := `chartname: repo/app
version: 1.0.0`
	rn := yaml.MustParse(doc)
	repos := map[string]string{"repo": "https://example.com"}
	_, err := UpdateK0sctlConfigchart(
		context.Background(),
		fakeHelmUpdater{latest: "1.1.0"},
		repos,
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	verNode, err := rn.Pipe(yaml.Get("version"))
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if yaml.GetValue(verNode) != "1.1.0" {
		t.Fatalf("unexpected version: %s", yaml.GetValue(verNode))
	}
}

func TestUpdateK0sctlConfigchart_EmptyLatestNoChange(t *testing.T) {
	doc := `chartname: repo/app
version: 1.0.0`
	rn := yaml.MustParse(doc)
	repos := map[string]string{"repo": "https://example.com"}
	_, err := UpdateK0sctlConfigchart(
		context.Background(),
		fakeHelmUpdater{latest: ""},
		repos,
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	verNode, err := rn.Pipe(yaml.Get("version"))
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if yaml.GetValue(verNode) != "1.0.0" {
		t.Fatalf("unexpected version: %s", yaml.GetValue(verNode))
	}
}

func TestUpdateK0sctlConfigchart_InvalidRepoURL(t *testing.T) {
	doc := `chartname: repo/app
version: 1.0.0`
	rn := yaml.MustParse(doc)
	repos := map[string]string{"repo": "invalid-url"}
	_, err := UpdateK0sctlConfigchart(
		context.Background(),
		fakeHelmUpdater{latest: "1.2.0"},
		repos,
	).Filter(rn)
	if err == nil {
		t.Fatalf("expected error for invalid repo URL")
	}
}

func TestUpdateK0sctlConfig_ProcessesCharts(t *testing.T) {
	doc := `spec:
  k0s:
    config:
      spec:
        extensions:
          helm:
            repositories:
            - name: repo
              url: https://example.com
            charts:
            - hartname: repo/app
              version: 1.0.0
            - chartname: repo/other
              version: 0.1.0`
	rn := yaml.MustParse(doc)
	_, err := UpdateK0sctlConfig(context.Background(), fakeHelmUpdater{latest: "2.0.0"}).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chartsNode, err := rn.Pipe(
		yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "charts"),
	)
	if err != nil {
		t.Fatalf("lookup charts: %v", err)
	}
	elems, err := chartsNode.Elements()
	if err != nil {
		t.Fatalf("elements: %v", err)
	}
	if len(elems) != 2 {
		t.Fatalf("unexpected charts length: %d", len(elems))
	}
	for i, e := range elems {
		verNode, err := e.Pipe(yaml.Get("version"))
		if err != nil {
			t.Fatalf("get version for chart %d: %v", i, err)
		}
		if yaml.GetValue(verNode) != "2.0.0" {
			t.Fatalf("unexpected version for chart %d: %s", i, yaml.GetValue(verNode))
		}
	}
}

func TestUpdateK0sctlConfig_NoCharts(t *testing.T) {
	doc := `spec:
  k0s:
    config:
      spec:
        extensions:
          helm:
            repositories:
            - name: repo
              url: https://example.com`
	rn := yaml.MustParse(doc)
	_, err := UpdateK0sctlConfig(context.Background(), fakeHelmUpdater{latest: "2.0.0"}).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
