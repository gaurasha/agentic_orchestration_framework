package credentials

import (
	"errors"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	key := []byte("k")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tok, err := sign(Claims{Tenant: "acme", Expires: now.Add(time.Hour)}, key)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := sign(Claims{Tenant: "acme", Expires: now.Add(time.Hour)}, []byte("other"))

	cases := []struct {
		name  string
		token string
		key   []byte
		now   time.Time
		want  string
		fails bool
	}{
		{"valid", tok, key, now, "acme", false},
		{"expired", tok, key, now.Add(2 * time.Hour), "", true},
		{"wrong key", other, key, now, "", true},
		{"tampered", tok[:len(tok)-2] + "xx", key, now, "", true},
		{"malformed", "not.a.jwt.really", key, now, "", true},
		{"empty", "", key, now, "", true},
	}
	for _, c := range cases {
		got, err := parse(c.token, c.key, c.now)
		if c.fails {
			if !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("%s: err = %v, want ErrUnauthenticated", c.name, err)
			}
			continue
		}
		if err != nil || got.Tenant != c.want {
			t.Errorf("%s: got %+v, %v; want tenant %q", c.name, got, err, c.want)
		}
	}
}
