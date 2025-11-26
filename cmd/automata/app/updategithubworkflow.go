package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/config"
	automatakio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/shikanime-studio/automata/internal/vsc"
	"github.com/spf13/cobra"
)

// NewUpdateGitHubWorkflowCmd creates the "githubworkflow" command that updates
// GitHub Actions versions in workflow files.
func NewUpdateGitHubWorkflowCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "githubworkflow DIR...",
		Short: "Update GitHub Actions in workflows to latest major versions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := []vsc.GitHubClientOption{}
			if tok := cfg.GitHubToken(); tok != "" {
				options = append(options, vsc.WithAuthToken(tok))
			}
			client := vsc.NewGitHubClient(options...)
			for _, a := range args {
				root := strings.TrimSpace(a)
				if root == "" {
					continue
				}
				if err := automatakio.UpdateGitHubWorkflows(cmd.Context(), client, root).Execute(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
