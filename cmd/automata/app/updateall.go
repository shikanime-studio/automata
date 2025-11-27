package app

import (
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/container"
	"github.com/shikanime-studio/automata/internal/github"
	"github.com/shikanime-studio/automata/internal/helm"
	ikio "github.com/shikanime-studio/automata/internal/kio"
)

// NewUpdateAllCmd returns a command that runs all update operations over directories.
func NewUpdateAllCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "all [DIR...]",
		Short: "Run all update operations",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cu := container.NewUpdater()
			hu := helm.NewUpdater()
			gu := github.NewUpdater(github.NewClient(cmd.Context(), cfg))

			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				g.Go(
					func() error {
						return ikio.UpdateKustomization(cmd.Context(), cu, r).Execute()
					},
				)
				g.Go(func() error {
					return runUpdateSops(cmd.Context(), r)
				})
				g.Go(func() error {
					return ikio.UpdateK0sctlConfigs(cmd.Context(), hu, r).Execute()
				})
				g.Go(func() error {
					return ikio.UpdateGitHubWorkflows(cmd.Context(), gu, r).Execute()
				})
				g.Go(func() error {
					return runUpdateScript(cmd.Context(), r)
				})
				return g.Wait()
			}
			return g.Wait()
		},
	}
}
