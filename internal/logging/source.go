package logging

import "context"

// Source lists containers and follows one container's logs.
type Source interface {
	Targets(ctx context.Context) ([]Target, error)
	Follow(ctx context.Context, id, since string, emit func(Entry) error) error
}
