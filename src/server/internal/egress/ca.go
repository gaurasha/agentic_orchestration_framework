package egress

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"time"
)

// CA is the platform's certificate authority. A sandbox trusts it, and
// nothing else does, so egress can answer a CONNECT for any host with a
// certificate for that host, read the request inside the TLS connection,
// and swap the placeholder there. That is what lets a CLI that fixes its
// own host, such as gh, run in the sandbox with only a placeholder. The
// key lives in this process and dies with it.
type CA struct {
	cert   *x509.Certificate
	key    *ecdsa.PrivateKey
	pem    []byte
	random io.Reader

	mu     sync.Mutex
	leaves map[string]*tls.Certificate
}

// NewCA makes a CA valid from now for life. Its leaves share that life:
// they are minted per process too, so expiry adds nothing.
func NewCA(random io.Reader, now time.Time, life time.Duration) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), random)
	if err != nil {
		return nil, fmt.Errorf("ca key: %w", err)
	}
	serial, err := serialNumber(random)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "egress"},
		NotBefore:             now,
		NotAfter:              now.Add(life),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(random, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("ca certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("ca certificate: %w", err)
	}
	return &CA{
		cert: cert, key: key, random: random,
		pem:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		leaves: map[string]*tls.Certificate{},
	}, nil
}

// PEM is the CA certificate, for a sandbox to trust. It holds no key.
func (c *CA) PEM() []byte { return c.pem }

// Leaf is a certificate for host, signed by the CA. host is a name or an
// IP, without a port. Leaves are cached, so a host costs one signing.
func (c *CA) Leaf(host string) (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if leaf, ok := c.leaves[host]; ok {
		return leaf, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), c.random)
	if err != nil {
		return nil, fmt.Errorf("leaf key for %s: %w", host, err)
	}
	serial, err := serialNumber(c.random)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    c.cert.NotBefore,
		NotAfter:     c.cert.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(c.random, tmpl, c.cert, &key.PublicKey, c.key)
	if err != nil {
		return nil, fmt.Errorf("leaf for %s: %w", host, err)
	}
	leaf := &tls.Certificate{Certificate: [][]byte{der, c.cert.Raw}, PrivateKey: key}
	c.leaves[host] = leaf
	return leaf, nil
}

// serialNumber is 128 random bits, as CAs are required to issue.
func serialNumber(random io.Reader) (*big.Int, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(random, b); err != nil {
		return nil, fmt.Errorf("serial number: %w", err)
	}
	return new(big.Int).SetBytes(b), nil
}
