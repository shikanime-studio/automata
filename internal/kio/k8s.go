package kio

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// RecommandedLabelsSetter sets Kubernetes recommended labels on resources.
type RecommandedLabelsSetter struct {
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version,omitempty"`
}

// Filter applies name and version labels to all label pairs.
func (s RecommandedLabelsSetter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	labelsNode, err := rn.Pipe(yaml.Lookup("labels"))
	if err != nil {
		return rn, fmt.Errorf("lookup labels: %w", err)
	}
	labelNodes, err := labelsNode.Elements()
	if err != nil {
		return rn, fmt.Errorf("get labels elements: %w", err)
	}
	for _, labelNode := range labelNodes {
		if err := labelNode.PipeE(
			yaml.Lookup("pairs"),
			yaml.SetField("app.kubernetes.io/name", yaml.NewStringRNode(s.Name))); err != nil {
			return rn, fmt.Errorf("set label %s: %w", "app.kubernetes.io/name", err)
		}
		if err := labelNode.PipeE(
			yaml.Lookup("pairs"),
			yaml.SetField("app.kubernetes.io/version", yaml.NewStringRNode(s.Version))); err != nil {
			return rn, fmt.Errorf("set label %s: %w", "app.kubernetes.io/version", err)
		}
	}
	return rn, nil
}

// SetRecommandedLabels creates a setter for Kubernetes recommended labels.
func SetRecommandedLabels(name, version string) RecommandedLabelsSetter {
	return RecommandedLabelsSetter{Name: name, Version: version}
}
