package app

import (
	"strings"

	automatakio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/spf13/cobra"
)

// NewUpdateKustomizationCmd updates kustomize image tags across a directory tree.
// It scans for kustomization.yaml files and updates image tags based on
// the images annotation configuration and chosen registry strategy.
func NewUpdateKustomizationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kustomization DIR...",
		Short: "Update kustomize image tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			for _, a := range args {
				root := strings.TrimSpace(a)
				if root == "" {
					continue
				}
				if err := automatakio.UpdateKustomization(root).Execute(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
