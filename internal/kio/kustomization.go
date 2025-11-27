package kio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/shikanime-studio/automata/internal/container"
	update "github.com/shikanime-studio/automata/internal/updater"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// UpdateKustomization creates a kustomize pipeline to update image tags
// and recommended labels for images defined in the kustomization.yaml at the given directory.
func UpdateKustomization(
	ctx context.Context,
	u update.Updater[*container.ImageRef],
	path string,
) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"kustomization.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateKustomizationsImages(ctx, u),
			UpdateKustomizationsLabels(),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: path},
		},
	}
}

// UpdateKustomizationsImages runs image tag updates across kustomization files.
func UpdateKustomizationsImages(
	ctx context.Context,
	u update.Updater[*container.ImageRef],
) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := node.PipeE(UpdateKustomizationImages(ctx, u)); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

// UpdateKustomizationImages updates image tags for one kustomization.
func UpdateKustomizationImages(
	ctx context.Context,
	u update.Updater[*container.ImageRef],
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		imageAnnotationNode, err := node.Pipe(GetImagesAnnotation())
		if err != nil {
			return nil, fmt.Errorf("get images annotation: %w", err)
		}
		imageConfigsByName, err := GetKustomizationImagesConfig(imageAnnotationNode)
		if err != nil {
			return nil, fmt.Errorf("get image config: %w", err)
		}

		imagesNode, err := node.Pipe(yaml.Lookup("images"))
		if err != nil {
			return nil, fmt.Errorf("lookup images: %w", err)
		}
		imageNodes, err := imagesNode.Elements()
		if err != nil {
			return nil, fmt.Errorf("get images elements: %w", err)
		}

		for _, img := range imageNodes {
			nameNode, err := img.Pipe(yaml.Get("name"))
			if err != nil {
				slog.Warn("missing name in images entry", "err", err)
				continue
			}
			name := yaml.GetValue(nameNode)
			cfg, ok := imageConfigsByName[name]
			if !ok {
				continue
			}
			newNameNode, err := img.Pipe(yaml.Get("newName"))
			if err != nil {
				return nil, fmt.Errorf("get newName for %s: %w", name, err)
			}

			options := []update.Option{}
			if cfg.Transform != nil {
				options = append(options, update.WithTransform(cfg.Transform))
			}

			imageRef := container.ImageRef{Name: yaml.GetValue(newNameNode)}

			newTagNode, err := img.Pipe(yaml.Get("newTag"))
			if err != nil {
				return nil, fmt.Errorf("get current newTag for %s: %w", name, err)
			}
			newTag := yaml.GetValue(newTagNode)
			if newTag != "" {
				imageRef.Tag = newTag
			}

			excludes := map[string]struct{}{}
			for _, e := range cfg.Excludes {
				excludes[e] = struct{}{}
			}
			latest, err := u.Update(ctx, &imageRef, options...)
			if err != nil {
				return nil, fmt.Errorf("find latest tag: %w", err)
			}
			if latest == "" {
				continue
			}
			if _, excluded := excludes[latest]; excluded {
				continue
			}
			if err = img.PipeE(yaml.SetField("newTag", yaml.NewStringRNode(latest))); err != nil {
				return nil, fmt.Errorf("set newTag for %s: %w", name, err)
			}
			slog.Info("updated image tag", "name", name, "image", imageRef.String(), "tag", latest)
		}
		return node, nil
	})
}

// UpdateKustomizationsLabels sets recommended labels across kustomization files.
func UpdateKustomizationsLabels() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := node.PipeE(UpdateKustomizationLabelsNode()); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

// UpdateKustomizationLabelsNode sets recommended labels for one kustomization.
func UpdateKustomizationLabelsNode() yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		imageAnnotationNode, err := node.Pipe(GetImagesAnnotation())
		if err != nil {
			return nil, fmt.Errorf("get images annotation: %w", err)
		}
		imageConfigsByName, err := GetKustomizationImagesConfig(imageAnnotationNode)
		if err != nil {
			return nil, fmt.Errorf("get image config: %w", err)
		}

		labelsNode, err := node.Pipe(yaml.Lookup("labels"))
		if err != nil {
			return nil, fmt.Errorf("lookup labels: %w", err)
		}
		labelNodes, err := labelsNode.Elements()
		if err != nil {
			return nil, fmt.Errorf("get labels pairs: %w", err)
		}

		for _, labelNode := range labelNodes {
			pairNode, err := labelNode.Pipe(yaml.Lookup("pairs"), yaml.Get(KubernetesNameLabel))
			if err != nil {
				return nil, fmt.Errorf("get %s: %w", KubernetesNameLabel, err)
			}
			pair := yaml.GetValue(pairNode)

			imagesNode, err := node.Pipe(yaml.Lookup("images"))
			if err != nil {
				return nil, fmt.Errorf("lookup images: %w", err)
			}
			imageNodes, err := imagesNode.Elements()
			if err != nil {
				return nil, fmt.Errorf("get images elements: %w", err)
			}

			for _, img := range imageNodes {
				nameNode, err := img.Pipe(yaml.Get("name"))
				if err != nil {
					slog.Warn("missing name in images entry", "err", err)
					continue
				}
				name := yaml.GetValue(nameNode)

				cfg, ok := imageConfigsByName[name]
				if !ok {
					continue
				}

				if pair != name {
					continue
				}

				newTagNode, err := img.Pipe(yaml.Get("newTag"))
				if err != nil {
					return nil, fmt.Errorf("get current newTag for %s: %w", name, err)
				}
				newTag := yaml.GetValue(newTagNode)
				if newTag == "" {
					continue
				}

				vers, err := update.Canonical(newTag, update.WithTransform(cfg.Transform))
				if err != nil {
					return nil, fmt.Errorf("parse semver for %s: %w", newTag, err)
				}
				if err = node.PipeE(SetRecommandedLabels(name, vers)); err != nil {
					return nil, fmt.Errorf("set %s: %w", KubernetesVersionLabel, err)
				}
				slog.Info("updated recommended labels", "name", name, "image", name, "tag", vers)
			}
		}
		return node, nil
	})
}

// Kustomization constants for annotations and label keys.
const (
	ImagesAnnotation       = "automata.shikanime.studio/images"
	KubernetesNameLabel    = "app.kubernetes.io/name"
	KubernetesVersionLabel = "app.kubernetes.io/version"
)

// GetImagesAnnotation retrieves the image config annotation.
func GetImagesAnnotation() yaml.Filter {
	return yaml.GetAnnotation(ImagesAnnotation)
}

// KustomizationImagesEntrySetter sets fields on an images entry.
type KustomizationImagesEntrySetter struct {
	Name    string `yaml:"name,omitempty"`
	NewName string `yaml:"newName,omitempty"`
	NewTag  string `yaml:"newTag,omitempty"`
}

// Filter applies the setter to the provided kustomization node.
func (s KustomizationImagesEntrySetter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	images, err := rn.Pipe(yaml.PathGetter{
		Path: []string{"images"}, Create: yaml.MappingNode,
	})
	if err != nil || yaml.IsMissingOrNull(images) {
		return rn, err
	}
	if s.Name != "" {
		if err := images.PipeE(
			yaml.MatchElement("name", s.Name),
			yaml.SetField("name", yaml.NewStringRNode(s.Name))); err != nil {
			return rn, err
		}
	}
	if s.NewName != "" {
		if err := images.PipeE(
			yaml.MatchElement("name", s.Name),
			yaml.SetField("newName", yaml.NewStringRNode(s.NewName))); err != nil {
			return rn, err
		}
	}
	if s.NewTag != "" {
		if err := images.PipeE(
			yaml.MatchElement("name", s.Name),
			yaml.SetField("newTag", yaml.NewStringRNode(s.NewTag))); err != nil {
			return rn, err
		}
	}
	return rn, nil
}

// SetKustomizationImage creates a setter to update an image entry.
func SetKustomizationImage(name, newName, newTag string) KustomizationImagesEntrySetter {
	return KustomizationImagesEntrySetter{Name: name, NewName: newName, NewTag: newTag}
}

// KustomizationImagesConfig describes image update behavior from annotation.
type KustomizationImagesConfig struct {
	Name      string
	Transform *regexp.Regexp
	Excludes  []string
}

// UnmarshalJSON parses the JSON representation of KustomizationImagesConfig.
func (c *KustomizationImagesConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name        string   `json:"name"`
		TagRegex    string   `json:"tag-regex"`
		ExcludeTags []string `json:"exclude-tags"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Name = raw.Name

	if raw.TagRegex != "" {
		re, err := regexp.Compile(raw.TagRegex)
		if err != nil {
			return fmt.Errorf("invalid tag-regex %q: %w", raw.TagRegex, err)
		}
		c.Transform = re
	}

	if len(raw.ExcludeTags) > 0 {
		c.Excludes = raw.ExcludeTags
	}

	return nil
}

// GetKustomizationImagesConfig reads image config from the annotation node.
func GetKustomizationImagesConfig(node *yaml.RNode) (map[string]KustomizationImagesConfig, error) {
	if yaml.IsMissingOrNull(node) {
		return nil, nil
	}
	var imageConfigs []KustomizationImagesConfig
	if err := json.Unmarshal([]byte(node.YNode().Value), &imageConfigs); err != nil {
		return nil, fmt.Errorf("unmarshal ImageConfig from annotation: %w", err)
	}
	cfgByName := make(map[string]KustomizationImagesConfig, len(imageConfigs))
	for _, c := range imageConfigs {
		cfgByName[c.Name] = c
	}
	return cfgByName, nil
}
