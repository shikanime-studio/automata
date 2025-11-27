package kio

import (
	"context"
	"testing"

	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/shikanime-studio/automata/internal/container"
	update "github.com/shikanime-studio/automata/internal/updater"
)

type fakeImageUpdater struct {
	latest string
	err    error
}

func (f fakeImageUpdater) Update(
	_ context.Context,
	_ *container.ImageRef,
	_ ...update.Option,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.latest, nil
}

func TestUpdateKustomizationImages_ExcludeTags(t *testing.T) {
	doc := `annotations:
  automata.shikanime.studio/images: '[{"name":"app","exclude-tags":["dev"]}]'
images:
- name: app
  newName: repo/app
  newTag: old`
	rn := yaml.MustParse(doc)
	_, err := UpdateKustomizationImages(
		context.Background(),
		fakeImageUpdater{latest: "dev"},
	).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	imgs, err := rn.Pipe(yaml.Lookup("images"))
	if err != nil {
		t.Fatalf("lookup images: %v", err)
	}
	elems, err := imgs.Elements()
	if err != nil {
		t.Fatalf("elements: %v", err)
	}
	if len(elems) != 1 {
		t.Fatalf("unexpected images length: %d", len(elems))
	}
	newTagNode, err := elems[0].Pipe(yaml.Get("newTag"))
	if err != nil {
		t.Fatalf("get newTag: %v", err)
	}
	if yaml.GetValue(newTagNode) != "old" {
		t.Fatalf("unexpected newTag: %s", yaml.GetValue(newTagNode))
	}
}

func TestUpdateKustomizationLabelsNode_CanonicalTransform(t *testing.T) {
	doc := `metadata:
  annotations:
    automata.shikanime.studio/images: '[{"name":"app","tag-regex":"^release-(?P<major>\\d+)-(?P<minor>\\d+)-(?P<patch>\\d+)$"}]'
labels:
- pairs:
    app.kubernetes.io/name: app
    app.kubernetes.io/version: old
images:
- name: app
  newName: repo/app
  newTag: release-1-2-3`
	rn := yaml.MustParse(doc)
	_, err := UpdateKustomizationLabelsNode(context.Background()).Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labelsNode, err := rn.Pipe(yaml.Lookup("labels"))
	if err != nil {
		t.Fatalf("lookup labels: %v", err)
	}
	elems, err := labelsNode.Elements()
	if err != nil {
		t.Fatalf("elements: %v", err)
	}
	if len(elems) != 1 {
		t.Fatalf("unexpected labels length: %d", len(elems))
	}
	nameNode, err := elems[0].Pipe(yaml.Lookup("pairs"), yaml.Get(KubernetesNameLabel))
	if err != nil {
		t.Fatalf("get name label: %v", err)
	}
	verNode, err := elems[0].Pipe(yaml.Lookup("pairs"), yaml.Get(KubernetesVersionLabel))
	if err != nil {
		t.Fatalf("get version label: %v", err)
	}
	if yaml.GetValue(nameNode) != "app" {
		t.Fatalf("unexpected name label: %s", yaml.GetValue(nameNode))
	}
	if yaml.GetValue(verNode) != "v1.2.3" {
		t.Fatalf("unexpected version label: %s", yaml.GetValue(verNode))
	}
}

func TestSetKustomizationImage_SetsFields(t *testing.T) {
	doc := `images:
- name: app
  newName: repo/app
  newTag: old
- name: other
  newName: repo/other
  newTag: 0.1.0
`
	rn := yaml.MustParse(doc)
	_, err := SetKustomizationImage("app", "repo/app", "1.2.3").Filter(rn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	imgs, err := rn.Pipe(yaml.Lookup("images"))
	if err != nil {
		t.Fatalf("lookup images: %v", err)
	}
	elems, err := imgs.Elements()
	if err != nil {
		t.Fatalf("elements: %v", err)
	}
	var found bool
	for _, e := range elems {
		nameNode, err := e.Pipe(yaml.Get("name"))
		if err != nil {
			t.Fatalf("get name: %v", err)
		}
		if yaml.GetValue(nameNode) == "app" {
			found = true
			newNameNode, err := e.Pipe(yaml.Get("newName"))
			if err != nil {
				t.Fatalf("get newName: %v", err)
			}
			newTagNode, err := e.Pipe(yaml.Get("newTag"))
			if err != nil {
				t.Fatalf("get newTag: %v", err)
			}
			if yaml.GetValue(newNameNode) != "repo/app" {
				t.Fatalf("unexpected newName: %s", yaml.GetValue(newNameNode))
			}
			if yaml.GetValue(newTagNode) != "1.2.3" {
				t.Fatalf("unexpected newTag: %s", yaml.GetValue(newTagNode))
			}
		}
	}
	if !found {
		t.Fatalf("target image not found")
	}
}

func TestGetKustomizationImagesConfig_ParsesJSON(t *testing.T) {
	raw := []byte(
		`[{"name":"app","tag-regex":"^(?P<major>\\d+)\\.(?P<minor>\\d+)\\.(?P<patch>\\d+)$","exclude-tags":["latest","dev"]}]`,
	)
	node := yaml.NewStringRNode(string(raw))
	m, err := GetKustomizationImagesConfig(node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, ok := m["app"]
	if !ok {
		t.Fatalf("config not found for app")
	}
	if c.Name != "app" {
		t.Fatalf("unexpected name: %s", c.Name)
	}
	if c.Transform == nil {
		t.Fatalf("expected transform regex")
	}
	if len(c.Excludes) != 2 {
		t.Fatalf("unexpected excludes length: %d", len(c.Excludes))
	}
}

func TestGetKustomizationImagesConfig_InvalidRegex(t *testing.T) {
	raw := []byte(`[{"name":"app","tag-regex":"(","exclude-tags":[]}]`)
	node := yaml.NewStringRNode(string(raw))
	_, err := GetKustomizationImagesConfig(node)
	if err == nil {
		t.Fatalf("expected error for invalid regex")
	}
}
