package sandbox

import (
	"bytes"
	"sort"
	"strings"
)

// EnvList turns an environment map into KEY=value pairs, sorted, so a
// runtime passes exactly this and nothing from its own environment.
func EnvList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// Capped collects at most MaxOutput bytes of a stream and drops the rest
// as it arrives, so a noisy command cannot grow the server's memory. A
// runtime gives one to the command's stdout and another to its stderr.
type Capped struct {
	buf     bytes.Buffer
	dropped bool
}

func (c *Capped) Write(p []byte) (int, error) {
	room := MaxOutput - c.buf.Len()
	switch {
	case len(p) <= room:
		c.buf.Write(p)
	case room > 0:
		c.buf.Write(p[:room])
		c.dropped = true
	case len(p) > 0:
		c.dropped = true
	}
	return len(p), nil
}

// String is the collected output, readable, marked when more was dropped.
func (c *Capped) String() string {
	s := strings.ToValidUTF8(c.buf.String(), "\uFFFD")
	if c.dropped {
		s += "…[truncated]"
	}
	return s
}
