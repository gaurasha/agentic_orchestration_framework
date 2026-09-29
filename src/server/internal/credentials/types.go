package credentials

import (
	"fmt"
	"time"
)

// Claims are what a token proves about its holder.
type Claims struct {
	Tenant  string
	Expires time.Time
}

// Placeholder is what a placeholder value stands for, and for how long.
type Placeholder struct {
	Tenant    string
	Execution string
	Ref       string   // the credential it stands for
	Hosts     []string // the only hosts it may be exchanged for
	Expires   time.Time
}

// PlaceholderPrefix starts every placeholder value, so the egress endpoint
// can find one in a request.
const PlaceholderPrefix = "fake_"

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

// MarshalJSON hides the value from any JSON response.
func (s Secret) MarshalJSON() ([]byte, error) { return []byte(`"[secret]"`), nil }
