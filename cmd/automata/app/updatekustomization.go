package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/container"
	ikio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewUpdateKustomizationCmd updates kustomize image tags across a directory tree.
// It scans for kustomization.yaml files and updates image tags based on
// the images annotation configuration and chosen registry strategy.
func NewUpdateKustomizationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kustomization [DIR...]",
		Short: "Update kustomize image tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := container.NewUpdater()
			var g errgroup.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				g.Go(
					func() error { return ikio.UpdateKustomization(cmd.Context(), u, r).Execute() },
				)
			}
			return g.Wait()
		},
	}
}
