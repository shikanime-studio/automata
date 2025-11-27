// Package app provides Cobra commands for automating update operations.
package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/container"
	"github.com/shikanime-studio/automata/internal/github"
	"github.com/shikanime-studio/automata/internal/helm"
	ikio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewUpdateCmd creates the umbrella "update" command and wires its subcommands.
// It shows help when invoked without a subcommand.
func NewUpdateCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewUpdateAllCmd(cfg))
	cmd.AddCommand(NewUpdateKustomizationCmd())
	cmd.AddCommand(NewUpdateSopsCmd())
	cmd.AddCommand(NewUpdateGitHubWorkflowCmd(cfg))
	cmd.AddCommand(NewUpdateK0sctlCmd())
	cmd.AddCommand(NewUpdateScriptCmd())
	cmd.AddCommand(NewUpdateFlakeCmd())
	return cmd
}

// NewUpdateAllCmd runs all update operations as a dedicated subcommand.
func NewUpdateAllCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "all [DIR...]",
		Short: "Run all update operations",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cu := container.NewUpdater()
			hu := helm.NewUpdater()
			gopts := []github.ClientOption{}
			if tok := cfg.GitHubToken(); tok != "" {
				gopts = append(gopts, github.WithAuthToken(tok))
			}
			gu := github.NewUpdater(github.NewClient(gopts...))

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
					return runUpdateSops(r)
				})
				g.Go(func() error {
					return ikio.UpdateK0sctlConfigs(cmd.Context(), hu, r).Execute()
				})
				g.Go(func() error {
					return ikio.UpdateGitHubWorkflows(cmd.Context(), gu, r).Execute()
				})
				g.Go(func() error {
					return runUpdateScript(r)
				})
				return g.Wait()
			}
			return g.Wait()
		},
	}
}
