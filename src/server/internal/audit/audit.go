// Package audit keeps an append-only record of every decision and outcome,
// one hash chain per tenant. Each record carries the hash of the one before
// it, so changing any past record breaks every hash after it.
//
// Needs: the interfaces in deps.go.
// Portability: interfaces only so far; no logic, service or adapters yet.
package audit

import (
	"context"
	"errors"
	"time"
)

// Errors, matched with errors.Is.
var (
	ErrNotFound = errors.New("no audit records") // the tenant's chain is empty
	ErrConflict = errors.New("audit conflict")   // another writer appended first
)

// API is what the audit log offers.
type API interface {
	// Append adds an event to its tenant's chain and returns the stored record.
	Append(ctx context.Context, e Event) (Record, error)
	// Verify recomputes a tenant's chain and returns an error naming the
	// first record that does not match.
	Verify(ctx context.Context, tenant string) error
}

// Event is one thing that happened.
type Event struct {
	Tenant    string
	Execution string
	Actor     string // the component that acted, e.g. "gateway"
	Action    string // what it did, e.g. "tool.call"
	Decision  string // "allow" or "deny"
	Reason    string
	Detail    map[string]string // more context; never a secret
}

// Record is an event as stored: timed, numbered and chained.
type Record struct {
	Event
	At   time.Time
	Seq  int64  // position in the tenant's chain, from 1
	Prev string // hash of the record before; empty for the first
	Hash string // hash of Prev and this record's fields
}
