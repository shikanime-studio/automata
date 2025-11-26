package kio

import (
	"fmt"
	"log/slog"

	"github.com/shikanime-studio/automata/internal/container"
	"github.com/shikanime-studio/automata/internal/utils"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// UpdateKustomization creates a kustomize pipeline to update image tags
// and recommended labels for images defined in the kustomization.yaml at the given directory.
func UpdateKustomization(path string) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"kustomization.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateKustomizationImages(),
			UpdateKustomizationLabels(),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: path},
		},
	}
}

func UpdateKustomizationImages() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := updateKustomizationImagesNode(node); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func updateKustomizationImagesNode(node *yaml.RNode) error {
	imageAnnotationNode, err := node.Pipe(utils.GetImagesAnnotation())
	if err != nil {
		return fmt.Errorf("get images annotation: %w", err)
	}
	imageConfigs, err := utils.GetImagesConfig(imageAnnotationNode)
	if err != nil {
		return fmt.Errorf("get image config: %w", err)
	}
	imageConfigsByName := utils.CreateImageConfigsByName(imageConfigs)

	imagesNode, err := node.Pipe(yaml.Lookup("images"))
	if err != nil {
		return fmt.Errorf("lookup images: %w", err)
	}
	imageNodes, err := imagesNode.Elements()
	if err != nil {
		return fmt.Errorf("get images elements: %w", err)
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
			return fmt.Errorf("get newName for %s: %w", name, err)
		}

		options := []container.FindLatestOption{}
		if cfg.StrategyType != utils.FullUpdate {
			options = append(options, container.WithStrategyType(cfg.StrategyType))
		}
		if len(cfg.ExcludeTags) > 0 {
			options = append(options, container.WithExclude(cfg.ExcludeTags))
		}
		if cfg.TagRegex != nil {
			options = append(options, container.WithTransform(cfg.TagRegex))
		}

		imageRef := container.ImageRef{Name: yaml.GetValue(newNameNode)}

		currentTagNode, err := img.Pipe(yaml.Get("newTag"))
		if err != nil {
			return fmt.Errorf("get current newTag for %s: %w", name, err)
		}
		currentTag := yaml.GetValue(currentTagNode)
		if currentTag != "" {
			var version string
			version, err = utils.ParseSemverWithRegex(cfg.TagRegex, currentTag)
			if err != nil {
				return fmt.Errorf("parse semver for %s: %w", currentTag, err)
			}
			imageRef.Tag = version
		}

		latest, err := container.FindLatestTag(&imageRef, options...)
		if err != nil {
			latest = currentTag
		}
		if latest == "" {
			slog.Info("no matching tag found", "image", name)
			continue
		}
		if err = img.PipeE(yaml.SetField("newTag", yaml.NewStringRNode(latest))); err != nil {
			return fmt.Errorf("set newTag for %s: %w", name, err)
		}
		slog.Info("updated image tag", "name", name, "image", imageRef.String(), "tag", latest)
	}
	return nil
}

func UpdateKustomizationLabels() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := updateKustomizationLabelsNode(node); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func updateKustomizationLabelsNode(node *yaml.RNode) error {
	imageAnnotationNode, err := node.Pipe(utils.GetImagesAnnotation())
	if err != nil {
		return fmt.Errorf("get images annotation: %w", err)
	}
	imageConfigs, err := utils.GetImagesConfig(imageAnnotationNode)
	if err != nil {
		return fmt.Errorf("get image config: %w", err)
	}
	imageConfigsByName := utils.CreateImageConfigsByName(imageConfigs)

	labelsNode, err := node.Pipe(yaml.Lookup("labels"))
	if err != nil {
		return fmt.Errorf("lookup labels: %w", err)
	}
	labelNodes, err := labelsNode.Elements()
	if err != nil {
		return fmt.Errorf("get labels pairs: %w", err)
	}
	var recoLabelName string
	for _, labelNode := range labelNodes {
		node, err = labelNode.Pipe(yaml.Lookup("pairs"), yaml.Get(utils.KubernetesNameLabel))
		if err != nil {
			return fmt.Errorf("get %s: %w", utils.KubernetesNameLabel, err)
		}
		recoLabelName = yaml.GetValue(node)
	}

	imagesNode, err := node.Pipe(yaml.Lookup("images"))
	if err != nil {
		return fmt.Errorf("lookup images: %w", err)
	}
	imageNodes, err := imagesNode.Elements()
	if err != nil {
		return fmt.Errorf("get images elements: %w", err)
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

		if recoLabelName == name {
			currentTagNode, err := img.Pipe(yaml.Get("newTag"))
			if err != nil {
				return fmt.Errorf("get current newTag for %s: %w", name, err)
			}
			latest := yaml.GetValue(currentTagNode)
			if latest == "" {
				continue
			}

			var vers string
			if cfg.TagRegex != nil {
				vers, err = utils.ParseSemverWithRegex(cfg.TagRegex, latest)
				if err != nil {
					return fmt.Errorf("parse semver for %s: %w", latest, err)
				}
			}
			if err = node.PipeE(utils.SetRecommandedLabels(name, utils.Canonical(vers))); err != nil {
				return fmt.Errorf("set %s: %w", utils.KubernetesVersionLabel, err)
			}
			slog.Info("updated recommended labels", "name", name, "image", name, "tag", latest)
		}
	}
	return nil
}
