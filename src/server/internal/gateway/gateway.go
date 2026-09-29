// Package gateway is the one path every tool call takes. For each call it
// loads the tool, checks that the caller's pinned agent version grants it,
// checks the arguments against the tool's schema and the grant's allowlist,
// returns the saved result if the call's key has run before, pauses for a
// human if the grant says so, checks the budget, runs the tool, and hides
// the credential from the result. The caller never holds the credential.
package gateway

import (
	"context"
	"errors"
	"log/slog"
)

// A refused call wraps one of these, with the reason in its message.
var (
	ErrDenied         = errors.New("denied")           // tool, argument or host not allowed
	ErrApprovalNeeded = errors.New("approval needed")  // a human must approve this call first
	ErrExhausted      = errors.New("budget exhausted") // the execution's budget is spent
	ErrConflict       = errors.New("conflict")         // the same key is still running
	// ErrUncertain wraps a failure after the call was sent: the effect of
	// the call is unknown. It is journaled, so a retry replays it.
	ErrUncertain = errors.New("uncertain result")
)

type API interface {
	Call(ctx context.Context, c Call) (Result, error)
	// Approve puts a human's approval on file for the call with this key,
	// so the next Call with it runs.
	Approve(ctx context.Context, c Caller, key string) error
}

func New(d Deps, cfg Config) API {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if cfg.MaxOutput <= 0 {
		cfg.MaxOutput = 4096
	}
	return &service{d: d, cfg: cfg}
}
