package credentials

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Tokens are JWTs signed with HS256, built with the standard library only.

const header = `{"alg":"HS256","typ":"JWT"}`

type payload struct {
	Tenant  string `json:"tnt"`
	Expires int64  `json:"exp"`
}

var b64 = base64.RawURLEncoding

func sign(c Claims, key []byte) (string, error) {
	body, err := json.Marshal(payload{Tenant: c.Tenant, Expires: c.Expires.Unix()})
	if err != nil {
		return "", err
	}
	signing := b64.EncodeToString([]byte(header)) + "." + b64.EncodeToString(body)
	return signing + "." + b64.EncodeToString(mac(signing, key)), nil
}

// parse checks the signature and expiry; every failure is ErrUnauthenticated.
func parse(token string, key []byte, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("%w: malformed token", ErrUnauthenticated)
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac(parts[0]+"."+parts[1], key)) {
		return Claims{}, fmt.Errorf("%w: bad signature", ErrUnauthenticated)
	}
	body, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: malformed token", ErrUnauthenticated)
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil || p.Tenant == "" {
		return Claims{}, fmt.Errorf("%w: malformed claims", ErrUnauthenticated)
	}
	c := Claims{Tenant: p.Tenant, Expires: time.Unix(p.Expires, 0)}
	if !now.Before(c.Expires) {
		return Claims{}, fmt.Errorf("%w: expired", ErrUnauthenticated)
	}
	return c, nil
}

func mac(s string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}

func validateStore(ref string, s Secret) error {
	if strings.TrimSpace(ref) == "" {
		return fmt.Errorf("%w: empty reference", ErrInvalid)
	}
	if s.Reveal() == "" {
		return fmt.Errorf("%w: empty value", ErrInvalid)
	}
	return nil
}

func validatePlaceholder(p Placeholder) error {
	if p.Tenant == "" || p.Execution == "" || p.Ref == "" {
		return fmt.Errorf("%w: a placeholder needs a tenant, an execution and a reference", ErrInvalid)
	}
	if len(p.Hosts) == 0 {
		return fmt.Errorf("%w: a placeholder needs the hosts it may reach", ErrInvalid)
	}
	return nil
}

// allows says whether a live placeholder may be exchanged for host now.
// Expiry is checked first, so an expired one never reveals its hosts.
func allows(p Placeholder, host string, now time.Time) error {
	if !now.Before(p.Expires) {
		return fmt.Errorf("%w: placeholder expired", ErrUnauthenticated)
	}
	if !slices.Contains(p.Hosts, host) {
		return fmt.Errorf("%w: placeholder is not for host %s", ErrDenied, host)
	}
	return nil
}
