package egress_test

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"testing"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress"
)

// 5.5: a leaf for a host verifies against the CA's certificate and names
// that host, whether a DNS name or an IP; the CA's PEM carries no key.
func TestLeafIsSignedForHost(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ca, err := egress.NewCA(rand.Reader, now, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca.PEM()) {
		t.Fatal("the CA's PEM holds no certificate")
	}
	if bytes.Contains(ca.PEM(), []byte("PRIVATE KEY")) {
		t.Fatal("the CA's PEM holds a key")
	}
	for _, host := range []string{"api.github.com", "127.0.0.1"} {
		leaf, err := ca.Leaf(host)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(leaf.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: host, Roots: roots, CurrentTime: now.Add(time.Hour)}); err != nil {
			t.Errorf("%s: leaf does not verify: %v", host, err)
		}
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: "other.example", Roots: roots, CurrentTime: now.Add(time.Hour)}); err == nil {
			t.Errorf("%s: leaf verifies for another host", host)
		}
		again, _ := ca.Leaf(host)
		if again != leaf {
			t.Errorf("%s: leaf was minted twice", host)
		}
	}
}
