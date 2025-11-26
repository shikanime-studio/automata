// Package app provides Cobra commands for automating update operations.
package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/vsc"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewUpdateCmd creates the umbrella "update" command and wires its
// subcommands. When invoked without flags, it shows help; with `--all`, it runs
// all update operations.
func NewUpdateCmd(cfg *config.Config) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
        Use:   "update DIR...",
        Short: "Update resources",
        Args:  cobra.MinimumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if !all {
                return cmd.Help()
            }
            for _, a := range args {
                root := strings.TrimSpace(a)
                if root == "" {
                    continue
                }

			var g errgroup.Group
			g.Go(func() error {
				return runUpdateKustomization(root)
			})
			g.Go(func() error {
				return runUpdateSops(root)
			})
			g.Go(func() error {
				return runUpdateK0sctl(root)
			})
			g.Go(func() error {
				options := []vsc.GitHubClientOption{}
				if tok := cfg.GitHubToken(); tok != "" {
					options = append(options, vsc.WithAuthToken(tok))
				}
				return runGitHubUpdateWorkflow(cmd.Context(), vsc.NewGitHubClient(options...), root)
			})
			g.Go(func() error {
				return runUpdateScript(root)
			})
			return g.Wait()
		},
	}
                var g errgroup.Group
                g.Go(func() error {
                    return automatakio.UpdateKustomization(root).Execute()
                })
                g.Go(func() error {
                    return runUpdateSops(root)
                })
                g.Go(func() error {
                    return automatakio.UpdateK0sctl(root).Execute()
                })
                g.Go(func() error {
                    options := []vsc.GitHubClientOption{}
                    if tok := cfg.GitHubToken(); tok != "" {
                        options = append(options, vsc.WithAuthToken(tok))
                    }
                    return automatakio.UpdateGitHubWorkflows(cmd.Context(), vsc.NewGitHubClient(options...), root).
                        Execute()
                })
                g.Go(func() error {
                    return runUpdateScript(root)
                })
                if err := g.Wait(); err != nil {
                    return err
                }
            }
            return nil
        },
    }
	cmd.Flags().BoolVar(&all, "all", false, "Run all update operations")
	cmd.AddCommand(NewUpdateKustomizationCmd())
	cmd.AddCommand(NewUpdateSopsCmd())
	cmd.AddCommand(NewUpdateGitHubWorkflowCmd(cfg))
	cmd.AddCommand(NewUpdateK0sctlCmd())
	cmd.AddCommand(NewUpdateScriptCmd())
	cmd.AddCommand(NewUpdateFlakeCmd())
	return cmd
}
