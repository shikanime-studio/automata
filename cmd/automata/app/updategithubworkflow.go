package app

import (
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/github"
	ikio "github.com/shikanime-studio/automata/internal/kio"
)

// NewUpdateGitHubWorkflowCmd creates the "githubworkflow" command that updates
// GitHub Actions versions in workflow files.
func NewUpdateGitHubWorkflowCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "githubworkflow [DIR...]",
		Short: "Update GitHub Actions in workflows to latest major versions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := github.NewUpdater(github.NewClient(cmd.Context(), cfg))
			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				g.Go(
					func() error { return ikio.UpdateGitHubWorkflows(cmd.Context(), u, r).Execute() },
				)
			}
			return g.Wait()
		},
	}
}
