package kio

import (
	"context"
	"strings"
	"testing"

	"github.com/shikanime-studio/automata/internal/github"
	update "github.com/shikanime-studio/automata/internal/updater"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type fakeUpdater struct {
	latest string
	err    error
}

func (f fakeUpdater) Update(
	_ context.Context,
	_ *github.ActionRef,
	_ ...update.Option,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.latest, nil
}

func TestUpdateGitHubWorkflowStep_UpdatesUses(t *testing.T) {
	doc := `uses: actions/checkout@v1`
	rn := yaml.MustParse(doc)
	_, err := UpdateGitHubWorkflowStep(
		context.Background(),
		fakeUpdater{latest: "v2"},
		"build",
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usesNode, err := rn.Pipe(yaml.Get("uses"))
	if err != nil {
		t.Fatalf("get uses: %v", err)
	}
	if yaml.GetValue(usesNode) != "actions/checkout@v2" {
		t.Fatalf("unexpected uses: %s", yaml.GetValue(usesNode))
	}
}

func TestUpdateGitHubWorkflowStep_NoUses(t *testing.T) {
	doc := `name: step`
	rn := yaml.MustParse(doc)
	_, err := UpdateGitHubWorkflowStep(
		context.Background(),
		fakeUpdater{latest: "v2"},
		"build",
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usesNode, err := rn.Pipe(yaml.Get("uses"))
	if err != nil {
		t.Fatalf("get uses: %v", err)
	}
	if usesNode != nil {
		t.Fatalf("expected no uses field")
	}
}

func TestUpdateGitHubWorkflowsAction_UpdatesJobs(t *testing.T) {
	doc := `jobs:
  build:
    steps:
    - uses: actions/setup-go@v5
  test:
    steps:
    - uses: actions/checkout@v1
`
	node := yaml.MustParse(doc)
	filter := UpdateGitHubWorkflowsAction(context.Background(), fakeUpdater{latest: "v6"})
	_, err := filter.Filter([]*yaml.RNode{node})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	jobs, err := node.Pipe(yaml.Lookup("jobs"))
	if err != nil {
		t.Fatalf("lookup jobs: %v", err)
	}
	for _, job := range []string{"build", "test"} {
		jn, err := jobs.Pipe(yaml.Lookup(job), yaml.Lookup("steps"))
		if err != nil {
			t.Fatalf("lookup steps for %s: %v", job, err)
		}
		elems, err := jn.Elements()
		if err != nil {
			t.Fatalf("elements for %s: %v", job, err)
		}
		for _, e := range elems {
			usesNode, err := e.Pipe(yaml.Get("uses"))
			if err != nil {
				t.Fatalf("get uses for %s: %v", job, err)
			}
			if !strings.HasSuffix(yaml.GetValue(usesNode), "@v6") {
				t.Fatalf("unexpected uses for %s: %s", job, yaml.GetValue(usesNode))
			}
		}
	}
}

func TestUpdateGitHubWorkflowStep_EmptyLatestNoChange(t *testing.T) {
	doc := `uses: actions/checkout@v1`
	rn := yaml.MustParse(doc)
	_, err := UpdateGitHubWorkflowStep(
		context.Background(),
		fakeUpdater{latest: ""},
		"build",
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	usesNode, err := rn.Pipe(yaml.Get("uses"))
	if err != nil {
		t.Fatalf("get uses: %v", err)
	}
	if yaml.GetValue(usesNode) != "actions/checkout@v1" {
		t.Fatalf("unexpected uses: %s", yaml.GetValue(usesNode))
	}
}

func TestUpdateGitHubWorkflowAction_NoJobs(t *testing.T) {
	doc := `name: ci`
	node := yaml.MustParse(doc)
	_, err := UpdateGitHubWorkflowAction(
		context.Background(),
		fakeUpdater{latest: "v2"},
	).Filter(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
