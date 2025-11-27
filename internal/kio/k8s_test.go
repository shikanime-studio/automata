package kio

import (
	"testing"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestRecommandedLabelsSetter_Filter(t *testing.T) {
	doc := `labels:
- pairs:
    app.kubernetes.io/name: old
    app.kubernetes.io/version: old
- pairs:
    other: x
`
	rn := yaml.MustParse(doc)
	_, err := SetRecommandedLabels("myapp", "v1.2.3").Filter(rn)
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
	for i, e := range elems {
		nameNode, err := e.Pipe(yaml.Lookup("pairs"), yaml.Get(KubernetesNameLabel))
		if err != nil {
			t.Fatalf("get name label for element %d: %v", i, err)
		}
		verNode, err := e.Pipe(yaml.Lookup("pairs"), yaml.Get(KubernetesVersionLabel))
		if err != nil {
			t.Fatalf("get version label for element %d: %v", i, err)
		}
		if yaml.GetValue(nameNode) != "myapp" {
			t.Fatalf("unexpected name label: %s", yaml.GetValue(nameNode))
		}
		if yaml.GetValue(verNode) != "v1.2.3" {
			t.Fatalf("unexpected version label: %s", yaml.GetValue(verNode))
		}
	}
}
