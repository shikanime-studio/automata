// Package app provides Cobra commands for automating update operations.
package app

import (
	"github.com/shikanime-studio/automata/internal/config"
	"github.com/spf13/cobra"
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
