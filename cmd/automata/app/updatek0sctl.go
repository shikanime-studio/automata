package app

import (
	"strings"

	automatakio "github.com/shikanime-studio/automata/internal/kio"
	"github.com/spf13/cobra"
)

// NewUpdateK0sctlCmd updates k0sctl clusters with the latest chart versions.
func NewUpdateK0sctlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "k0sctl DIR...",
		Short: "Update k0sctl with latest chart versions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			for _, a := range args {
				root := strings.TrimSpace(a)
				if root == "" {
					continue
				}
				if err := automatakio.UpdateK0sctl(root).Execute(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
