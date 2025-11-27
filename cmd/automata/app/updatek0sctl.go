package app

import (
	"strings"

	"github.com/shikanime-studio/automata/internal/helm"
	ikio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/spf13/cobra"
	errgrp "golang.org/x/sync/errgroup"
)

// NewUpdateK0sctlCmd updates k0sctl clusters with the latest chart versions.
func NewUpdateK0sctlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "k0sctl [DIR...]",
		Short: "Update k0sctl with latest chart versions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := helm.NewUpdater()
			var g errgrp.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				g.Go(
					func() error { return ikio.UpdateK0sctlConfigs(cmd.Context(), u, r).Execute() },
				)
			}
			return g.Wait()
		},
	}
}
