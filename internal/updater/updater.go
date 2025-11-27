// Package updater defines the generic Updater interface used by components.
package updater

import "context"

// Updater defines a type that can resolve an updated string for a target value.
type Updater[T any] interface {
	Update(ctx context.Context, v T, opts ...Option) (string, error)
}
