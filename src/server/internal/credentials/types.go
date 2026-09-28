package credentials

import (
	"fmt"
	"time"
)

// Claims are what a token proves about its holder.
type Claims struct {
	Tenant       string
	Execution    string
	AgentID      string
	AgentVersion int
	Expires      time.Time
}

// Secret holds a credential value. It prints as "[secret]" under every fmt
// verb, and keeps the value behind a pointer, so a struct holding it in an
// unexported field prints an address. Call Reveal only where the value is used.
type Secret struct{ p *string }

func NewSecret(v string) Secret { return Secret{p: &v} }

// Reveal returns the value, or "" for the zero Secret.
func (s Secret) Reveal() string {
	if s.p == nil {
		return ""
	}
	return *s.p
}

func (s Secret) String() string { return "[secret]" }

// Format prints the placeholder for every verb, including %#v and %d.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = fmt.Fprint(f, "[secret]") }
