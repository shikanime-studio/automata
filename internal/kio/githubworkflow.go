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
			UpdateGitHubWorkflowsActions(ctx, client),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: filepath.Join(path, ".github", "workflows"),
			},
		},
	}
}

func UpdateGitHubWorkflowsActions(ctx context.Context, client *vsc.GitHubClient) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := UpdateGitHubWorkflowsAction(ctx, client, node); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func UpdateGitHubWorkflowsAction(
	ctx context.Context,
	client *vsc.GitHubClient,
	node *yaml.RNode,
) error {
	jobsNode, err := node.Pipe(yaml.Lookup("jobs"))
	if err != nil {
		slog.Warn("failed to lookup jobs", "err", err)
		return fmt.Errorf("lookup jobs: %w", err)
	}
	if jobsNode == nil {
		slog.Info("no jobs found")
		return nil
	}
	jobNames, err := jobsNode.Fields()
	if err != nil {
		slog.Warn("failed to list jobs", "err", err)
		return fmt.Errorf("get job fields: %w", err)
	}
	for _, j := range jobNames {
		if err := UpdateGitHubWorkflowsJob(ctx, client, jobsNode, j); err != nil {
			slog.Warn("job processing error", "job", j, "err", err)
		}
	}
	return nil
}

func UpdateGitHubWorkflowsJob(
	ctx context.Context,
	client *vsc.GitHubClient,
	jobsNode *yaml.RNode,
	jobName string,
) error {
	jobNode, err := jobsNode.Pipe(yaml.Lookup(jobName))
	if err != nil || jobNode == nil {
		slog.Info("skip job without steps", "job", jobName)
		return nil
	}
	stepsNode, err := jobNode.Pipe(yaml.Lookup("steps"))
	if err != nil || stepsNode == nil {
		slog.Info("job has no steps", "job", jobName)
		return nil
	}
	stepElems, err := stepsNode.Elements()
	if err != nil {
		slog.Warn("failed to get steps", "job", jobName, "err", err)
		return fmt.Errorf("get steps: %w", err)
	}
	for idx, step := range stepElems {
		if err := UpdateGitHubWorkflowsStep(ctx, client, step, jobName, idx); err != nil {
			return fmt.Errorf("step processing error: %w", err)
		}
	}
	return nil
}

func UpdateGitHubWorkflowsStep(
	ctx context.Context,
	client *vsc.GitHubClient,
	step *yaml.RNode,
	jobName string,
	idx int,
) error {
	usesNode, err := step.Pipe(yaml.Get("uses"))
	if err != nil {
		return fmt.Errorf("get uses: %w", err)
	}
	if usesNode == nil {
		return nil
	}
	curr := strings.TrimSpace(yaml.GetValue(usesNode))
	if curr == "" {
		slog.Info("empty uses value", "job", jobName, "step_index", idx)
		return nil
	}
	actionRef, err := vsc.ParseGitHubActionRef(curr)
	if err != nil {
		return fmt.Errorf("parse action ref: %w", err)
	}
	latest, err := client.FindLatestActionTag(ctx, actionRef)
	if err != nil {
		return fmt.Errorf("find latest tag: %w", err)
	}
	if latest == "" {
		slog.Info("no suitable tag found", "action", actionRef.String())
		return nil
	}
	newActionRef := vsc.GitHubActionRef{
		Owner:   actionRef.Owner,
		Repo:    actionRef.Repo,
		Version: latest,
	}
	if err := step.PipeE(yaml.SetField("uses", yaml.NewStringRNode(newActionRef.String()))); err != nil {
		return fmt.Errorf("set uses for %s/%s: %w", actionRef.Owner, actionRef.Repo, err)
	}
	slog.Info(
		"updated action",
		"job",
		jobName,
		"action",
		fmt.Sprintf("%s/%s", actionRef.Owner, actionRef.Repo),
		"from",
		actionRef.Version,
		"to",
		latest,
	)
	return nil
}
