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

// NewMigrateCmd performs migration: checks upgrades and applies corrections for new versions.
func NewMigrateCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate [DIR...]",
		Short: "Check for upgrades and apply corrections to work with new versions",
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
				rr := r
				g.Go(
					func() error { return ikio.UpdateKustomization(cmd.Context(), cu, rr).Execute() },
				)
				g.Go(
					func() error { return ikio.UpdateK0sctlConfigs(cmd.Context(), hu, rr).Execute() },
				)
				g.Go(
					func() error { return ikio.UpdateGitHubWorkflows(cmd.Context(), gu, rr).Execute() },
				)
				g.Go(func() error { return runUpdateSops(cmd.Context(), rr) })
				g.Go(func() error { return runUpdateScript(cmd.Context(), rr) })
				g.Go(func() error { return runUpdateFlake(cmd.Context(), rr) })
			}
			return g.Wait()
		},
	}
}
