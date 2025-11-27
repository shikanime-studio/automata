// Package kio provides kyaml pipelines and filters for YAML-based updates.
package kio

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/vsc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// UpdateGitHubWorkflows builds a kyaml pipeline that rewrites a
// workflow directory, skipping git-ignored files.
// UpdateGitHubWorkflows builds a kyaml pipeline that rewrites a
// workflow directory, skipping git-ignored files.
func UpdateGitHubWorkflows(
	ctx context.Context,
	client *vsc.GitHubClient,
	path string,
) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    filepath.Join(path, ".github", "workflows"),
				MatchFilesGlob: []string{"*.yml", "*.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateGitHubWorkflowsAction(ctx, client),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: filepath.Join(path, ".github", "workflows"),
			},
		},
	}
}

func UpdateGitHubWorkflowsAction(ctx context.Context, client *vsc.GitHubClient) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := node.PipeE(UpdateGitHubWorkflowAction(ctx, client)); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

// UpdateGitHubWorkflowAction updates all jobs within a single workflow.
func UpdateGitHubWorkflowAction(
	ctx context.Context,
	client *vsc.GitHubClient,
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		jobsNode, err := node.Pipe(yaml.Lookup("jobs"))
		if err != nil {
			slog.Warn("failed to lookup jobs", "err", err)
			return nil, fmt.Errorf("lookup jobs: %w", err)
		}
		if jobsNode == nil {
			slog.Info("no jobs found")
			return node, nil
		}
		jobNames, err := jobsNode.Fields()
		if err != nil {
			slog.Warn("failed to list jobs", "err", err)
			return nil, fmt.Errorf("get job fields: %w", err)
		}
		for _, j := range jobNames {
			if err := jobsNode.PipeE(UpdateGitHubWorkflowJob(ctx, client, j)); err != nil {
				slog.Warn("job processing error", "job", j, "err", err)
			}
		}
		return node, nil
	})
}

// UpdateGitHubWorkflowJob updates all steps within the named job.
func UpdateGitHubWorkflowJob(
	ctx context.Context,
	client *vsc.GitHubClient,
	name string,
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		jobNode, err := node.Pipe(yaml.Lookup(name))
		if err != nil || jobNode == nil {
			slog.Info("skip job without steps", "job", name)
			return node, nil
		}
		stepsNode, err := jobNode.Pipe(yaml.Lookup("steps"))
		if err != nil || stepsNode == nil {
			slog.Info("job has no steps", "job", name)
			return node, nil
		}
		stepElems, err := stepsNode.Elements()
		if err != nil {
			slog.Warn("failed to get steps", "job", name, "err", err)
			return nil, fmt.Errorf("get steps: %w", err)
		}
		for _, step := range stepElems {
			if err := step.PipeE(UpdateGitHubWorkflowStep(ctx, client, name)); err != nil {
				return nil, fmt.Errorf("step processing error: %w", err)
			}
		}
		return node, nil
	})
}

// UpdateGitHubWorkflowStep updates a step's uses to the latest action tag.
func UpdateGitHubWorkflowStep(
	ctx context.Context,
	client *vsc.GitHubClient,
	name string,
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		usesNode, err := node.Pipe(yaml.Get("uses"))
		if err != nil {
			return nil, fmt.Errorf("get uses: %w", err)
		}
		if usesNode == nil {
			return node, nil
		}
		curr := strings.TrimSpace(yaml.GetValue(usesNode))
		if curr == "" {
			slog.Info("empty uses value", "job", name)
			return node, nil
		}
		actionRef, err := vsc.ParseGitHubActionRef(curr)
		if err != nil {
			return nil, fmt.Errorf("parse action ref: %w", err)
		}
		latest, err := client.FindLatestActionTag(ctx, actionRef)
		if err != nil {
			return nil, fmt.Errorf("find latest tag: %w", err)
		}
		if latest == "" {
			slog.Info("no suitable tag found", "action", actionRef.String())
			return node, nil
		}
		newActionRef := vsc.GitHubActionRef{
			Owner:   actionRef.Owner,
			Repo:    actionRef.Repo,
			Version: latest,
		}
		if err := node.PipeE(yaml.SetField("uses", yaml.NewStringRNode(newActionRef.String()))); err != nil {
			return nil, fmt.Errorf("set uses for %s/%s: %w", actionRef.Owner, actionRef.Repo, err)
		}
		slog.Info(
			"updated action",
			"job",
			name,
			"action",
			fmt.Sprintf("%s/%s", actionRef.Owner, actionRef.Repo),
			"from",
			actionRef.Version,
			"to",
			latest,
		)
		return node, nil
	})
}
