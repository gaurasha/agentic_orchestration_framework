package credentials

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestSecretNeverPrints(t *testing.T) {
	const value = "hunter2"
	s := NewSecret(value)
	type exported struct{ Token Secret }
	type unexported struct{ token Secret }

	args := []any{s, &s, exported{s}, unexported{s}, []Secret{s}, map[string]Secret{"k": s}}
	verbs := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%d"}
	for _, arg := range args {
		for _, verb := range verbs {
			got := fmt.Sprintf(verb, arg)
			if strings.Contains(got, value) || strings.Contains(got, hex.EncodeToString([]byte(value))) {
				t.Errorf("Sprintf(%q, %T) leaked the value: %q", verb, arg, got)
			}
		}
	}
}

func TestSecretReveal(t *testing.T) {
	cases := []struct {
		name string
		s    Secret
		want string
	}{
		{"value", NewSecret("hunter2"), "hunter2"},
		{"zero", Secret{}, ""},
	}
	for _, c := range cases {
		if got := c.s.Reveal(); got != c.want {
			t.Errorf("%s: Reveal() = %q, want %q", c.name, got, c.want)
		}
	}
}
