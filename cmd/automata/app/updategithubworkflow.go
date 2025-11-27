package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/config"
	ikio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/shikanime-studio/automata/internal/vsc"
	"github.com/spf13/cobra"
	errgrp "golang.org/x/sync/errgroup"
)

// NewUpdateGitHubWorkflowCmd creates the "githubworkflow" command that updates
// GitHub Actions versions in workflow files.
func NewUpdateGitHubWorkflowCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "githubworkflow [DIR...]",
		Short: "Update GitHub Actions in workflows to latest major versions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []vsc.GitHubClientOption{}
			if tok := cfg.GitHubToken(); tok != "" {
				options = append(options, vsc.WithAuthToken(tok))
			}
			client := vsc.NewGitHubClient(options...)
			var g errgrp.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				g.Go(
					func() error { return ikio.UpdateGitHubWorkflows(cmd.Context(), client, r).Execute() },
				)
			}
			return g.Wait()
		},
	}
}
