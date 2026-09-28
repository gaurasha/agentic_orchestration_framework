// Package timeline keeps each execution's trajectory: every model call,
// tool call, approval, answer and result, in order.
package timeline

import "context"

type API interface {
	Append(ctx context.Context, e Event) (Entry, error)
	List(ctx context.Context, tenant, execution string) ([]Entry, error)
}
