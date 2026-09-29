package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// normalize puts a definition in one canonical shape, so two saves of the
// same content hash the same: tools sorted by name, empty allowlists nil.
func normalize(def Definition) Definition {
	out := def
	out.Name = strings.TrimSpace(def.Name)
	out.Tools = make([]ToolGrant, len(def.Tools))
	for i, t := range def.Tools {
		g := t
		if len(g.Allowlist) == 0 {
			g.Allowlist = nil
		}
		out.Tools[i] = g
	}
	sort.Slice(out.Tools, func(i, j int) bool { return out.Tools[i].Name < out.Tools[j].Name })
	return out
}

// hash identifies a normalized definition's content. json.Marshal writes
// map keys sorted, so the allowlists are canonical too.
func hash(def Definition) string {
	b, _ := json.Marshal(def)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// validate checks what needs no I/O; the registry check is the service's.
func validate(def Definition) error {
	if def.Name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalid)
	}
	if def.Model == "" {
		return fmt.Errorf("%w: empty model", ErrInvalid)
	}
	if def.Budget.MaxCalls < 0 {
		return fmt.Errorf("%w: negative budget", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, t := range def.Tools {
		if t.Name == "" {
			return fmt.Errorf("%w: a tool grant has no name", ErrInvalid)
		}
		if seen[t.Name] {
			return fmt.Errorf("%w: tool %s granted twice", ErrInvalid, t.Name)
		}
		seen[t.Name] = true
	}
	return nil
}
