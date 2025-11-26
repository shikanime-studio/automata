// Package app provides Cobra commands for automating update operations.
package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/vsc"
	"github.com/spf13/cobra"
	errgrp "golang.org/x/sync/errgroup"
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
			var gg errgrp.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				rr := r
				gg.Go(func() error {
					var g errgrp.Group
					g.Go(func() error { return runUpdateKustomization(rr) })
					g.Go(func() error { return runUpdateSops(rr) })
					g.Go(func() error { return runUpdateK0sctl(rr) })
					g.Go(func() error {
						options := []vsc.GitHubClientOption{}
						if tok := cfg.GitHubToken(); tok != "" {
							options = append(options, vsc.WithAuthToken(tok))
						}
						return runGitHubUpdateWorkflow(
							cmd.Context(),
							vsc.NewGitHubClient(options...),
							rr,
						)
					})
					g.Go(func() error { return runUpdateScript(rr) })
					return g.Wait()
				})
			}
			return gg.Wait()
		},
	}
}
