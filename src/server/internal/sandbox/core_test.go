package sandbox

import (
	"strings"
	"testing"
)

// 6.4: output is bounded while it is collected, not after the command ends.
func TestCappedDropsExcessAsItArrives(t *testing.T) {
	var c Capped
	chunk := strings.Repeat("x", MaxOutput/4+1)
	for i := 0; i < 8; i++ {
		if n, err := c.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("write %d = %d, %v", i, n, err)
		}
	}
	if c.buf.Len() != MaxOutput {
		t.Errorf("buffered %d bytes, want exactly %d", c.buf.Len(), MaxOutput)
	}
	if got := c.String(); !strings.HasSuffix(got, "…[truncated]") || len(got) != MaxOutput+len("…[truncated]") {
		t.Errorf("String = %d bytes ending %q", len(got), got[len(got)-20:])
	}
	var small Capped
	_, _ = small.Write([]byte("hi"))
	if small.String() != "hi" {
		t.Errorf("small = %q", small.String())
	}
}
