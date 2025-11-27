// Package kio provides kyaml pipelines and filters for YAML-based updates.
package kio

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/github"
	update "github.com/shikanime-studio/automata/internal/updater"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// UpdateGitHubWorkflows builds a kyaml pipeline that rewrites a
// workflow directory, skipping git-ignored files.
// UpdateGitHubWorkflows builds a kyaml pipeline that rewrites a
// workflow directory, skipping git-ignored files.
func UpdateGitHubWorkflows(
	ctx context.Context,
	u update.Updater[*github.ActionRef],
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
			UpdateGitHubWorkflowsAction(ctx, u),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: filepath.Join(path, ".github", "workflows"),
			},
		},
	}
}

// UpdateGitHubWorkflowsAction applies action updates across workflow files.
func UpdateGitHubWorkflowsAction(
	ctx context.Context,
	u update.Updater[*github.ActionRef],
) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		g := errgroup.Group{}
		for _, node := range nodes {
			g.Go(func() error {
				if err := node.PipeE(UpdateGitHubWorkflowAction(ctx, u)); err != nil {
					return err
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		return nodes, nil
	})
}

// UpdateGitHubWorkflowAction updates all jobs within a single workflow.
func UpdateGitHubWorkflowAction(
	ctx context.Context,
	u update.Updater[*github.ActionRef],
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		jobsNode, err := node.Pipe(yaml.Lookup("jobs"))
		if err != nil {
			slog.Warn("failed to lookup jobs", "err", err)
			return nil, fmt.Errorf("lookup jobs: %w", err)
		}
		if jobsNode == nil {
			slog.InfoContext(ctx, "no jobs found")
			return node, nil
		}
		jobNames, err := jobsNode.Fields()
		if err != nil {
			slog.WarnContext(ctx, "failed to list jobs", "err", err)
			return nil, fmt.Errorf("get job fields: %w", err)
		}
		for _, j := range jobNames {
			if err := jobsNode.PipeE(UpdateGitHubWorkflowJob(ctx, u, j)); err != nil {
				slog.WarnContext(ctx, "job processing error", "job", j, "err", err)
			}
		}
		return node, nil
	})
}

// UpdateGitHubWorkflowJob updates all steps within the named job.
func UpdateGitHubWorkflowJob(
	ctx context.Context,
	u update.Updater[*github.ActionRef],
	name string,
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		jobNode, err := node.Pipe(yaml.Lookup(name))
		if err != nil || jobNode == nil {
			slog.InfoContext(ctx, "skip job without steps", "job", name)
			return node, nil
		}
		stepsNode, err := jobNode.Pipe(yaml.Lookup("steps"))
		if err != nil || stepsNode == nil {
			return node, nil
		}
		stepElems, err := stepsNode.Elements()
		if err != nil {
			return nil, fmt.Errorf("get steps: %w", err)
		}
		for _, step := range stepElems {
			if err := step.PipeE(UpdateGitHubWorkflowStep(ctx, u, name)); err != nil {
				return nil, fmt.Errorf("step processing error: %w", err)
			}
		}
		return node, nil
	})
}

// UpdateGitHubWorkflowStep updates a step's uses to the latest action tag.
func UpdateGitHubWorkflowStep(
	ctx context.Context,
	u update.Updater[*github.ActionRef],
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
			slog.InfoContext(ctx, "empty uses value", "job", name)
			return node, nil
		}
		actionRef, err := github.ParseActionRef(curr)
		if err != nil {
			return nil, fmt.Errorf("parse action ref: %w", err)
		}
		latest, err := u.Update(ctx, actionRef)
		if err != nil {
			return nil, fmt.Errorf("find latest tag: %w", err)
		}
		if latest == "" {
			slog.InfoContext(ctx, "no suitable tag found", "action", actionRef.String())
			return node, nil
		}
		newActionRef := github.ActionRef{
			Owner:   actionRef.Owner,
			Repo:    actionRef.Repo,
			Version: latest,
		}
		if err := node.PipeE(yaml.SetField("uses", yaml.NewStringRNode(newActionRef.String()))); err != nil {
			return nil, fmt.Errorf("set uses for %s/%s: %w", actionRef.Owner, actionRef.Repo, err)
		}
		slog.InfoContext(ctx,
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
