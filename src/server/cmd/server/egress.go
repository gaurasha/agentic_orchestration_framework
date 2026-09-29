package main

// The egress endpoint, on its own listener, in two shapes that share one
// check-and-swap path:
//
//   - the path form: `METHOD /{scheme}/{host}/{path}` with the placeholder
//     in a header, for a tool that lets its base URL be set (git, curl);
//   - the proxy form: the sandbox has HTTPS_PROXY set to this listener, so
//     a CLI that fixes its host, such as gh, sends `CONNECT host:443`. The
//     listener answers with a certificate for that host signed by the
//     platform CA, which only the sandbox trusts, reads each request inside
//     the TLS connection, and forwards it like any other. A plain-http
//     proxy request (`GET http://host/path`) is forwarded the same way.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress"
)

const (
	egressMaxBody = 1 << 20
	egressTimeout = 60 * time.Second // per forwarded request
	connectIdle   = 60 * time.Second // a CONNECT tunnel with nothing to read is closed
)

func (a *app) egressHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("/{scheme}/{host}/", a.forward)
	mux.HandleFunc("/{scheme}/{host}", a.forward)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodConnect:
			a.connect(w, r)
		case r.URL.Host != "" && r.URL.Host != a.egressHost:
			a.forwardPlain(w, r)
		default:
			// Origin-form, or addressed to egress itself through the proxy:
			// the path form.
			mux.ServeHTTP(w, r)
		}
	})
}

// forward serves the path form.
func (a *app) forward(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	scheme, host := r.PathValue("scheme"), r.PathValue("host")
	prefix := "/" + scheme + "/" + host
	path := strings.TrimPrefix(r.URL.Path, prefix)
	rawPath := strings.TrimPrefix(r.URL.EscapedPath(), prefix)
	if path == "" {
		path, rawPath = "/", ""
	}
	a.writeReply(w, a.forwardReply(r.Context(), egress.Request{
		Method: r.Method, Scheme: scheme, Host: host, Path: path, RawPath: rawPath, RawQuery: r.URL.RawQuery, Header: r.Header, Body: body,
	}))
}

// forwardPlain serves a proxy request for a plain http URL.
func (a *app) forwardPlain(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	a.writeReply(w, a.forwardReply(r.Context(), egress.Request{
		Method: r.Method, Scheme: r.URL.Scheme, Host: stripDefaultPort(r.URL.Host, "80"),
		Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery, Header: r.Header, Body: body,
	}))
}

// connect serves a CONNECT tunnel: TLS with the platform CA's certificate
// for the host, then each request inside it is forwarded over https.
func (a *app) connect(w http.ResponseWriter, r *http.Request) {
	host := stripDefaultPort(r.Host, "443")
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	leaf, err := a.ca.Leaf(name)
	if err != nil {
		a.log.Error("egress certificate failed", "host", host, "err", err)
		writeError(w, http.StatusBadGateway, "no certificate for the host")
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, http.StatusInternalServerError, "CONNECT is not supported here")
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	tc := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{*leaf},
		NextProtos:   []string{"http/1.1"}, // no h2: one request at a time, read as text
		MinVersion:   tls.VersionTLS12,
	})
	defer tc.Close()
	a.log.Info("egress connect", "host", host)
	br := bufio.NewReader(tc)
	for {
		_ = tc.SetReadDeadline(time.Now().Add(connectIdle))
		req, err := http.ReadRequest(br)
		if err != nil {
			return // the client is done, or gave up
		}
		body, err := io.ReadAll(io.LimitReader(req.Body, egressMaxBody+1))
		_ = req.Body.Close()
		var rep reply
		if err != nil || len(body) > egressMaxBody {
			rep = errorReply(http.StatusRequestEntityTooLarge, "body too large", nil)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), egressTimeout)
			rep = a.forwardReply(ctx, egress.Request{
				Method: req.Method, Scheme: "https", Host: host, Path: req.URL.Path, RawPath: req.URL.RawPath, RawQuery: req.URL.RawQuery, Header: req.Header, Body: body,
			})
			cancel()
		}
		res := &http.Response{
			StatusCode: rep.Status, ProtoMajor: 1, ProtoMinor: 1, Header: rep.Header,
			Body: io.NopCloser(bytes.NewReader(rep.Body)), ContentLength: int64(len(rep.Body)), Close: req.Close,
		}
		_ = tc.SetWriteDeadline(time.Now().Add(connectIdle))
		if err := res.Write(tc); err != nil || req.Close {
			return
		}
	}
}

// forwardReply asks egress to forward and turns its answer, or refusal, into an
// HTTP reply. A 401 carries a challenge, so a client such as git that
// probes without credentials first retries with the ones it holds.
func (a *app) forwardReply(ctx context.Context, r egress.Request) reply {
	res, err := a.egress.Forward(ctx, r)
	switch {
	case errors.Is(err, egress.ErrUnauthenticated):
		return errorReply(http.StatusUnauthorized, err.Error(), http.Header{"WWW-Authenticate": {`Basic realm="egress"`}})
	case errors.Is(err, egress.ErrDenied):
		return errorReply(http.StatusForbidden, err.Error(), nil)
	case errors.Is(err, egress.ErrTooLarge):
		return errorReply(http.StatusBadGateway, err.Error(), nil)
	case err != nil:
		a.log.Error("egress failed", "err", err)
		return errorReply(http.StatusBadGateway, "upstream failed", nil)
	}
	return reply{Status: res.Status, Header: res.Header, Body: res.Body}
}

// reply is an HTTP reply to the sandbox, written to a ResponseWriter or
// into a CONNECT tunnel.
type reply struct {
	Status int
	Header http.Header
	Body   []byte
}

func errorReply(status int, msg string, hdr http.Header) reply {
	if hdr == nil {
		hdr = http.Header{}
	}
	hdr.Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]string{"error": msg})
	return reply{Status: status, Header: hdr, Body: b}
}

func (a *app) writeReply(w http.ResponseWriter, r reply) {
	for k, vs := range r.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(r.Status)
	_, _ = w.Write(r.Body)
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, egressMaxBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "body too large")
		return nil, false
	}
	return body, true
}

// stripDefaultPort drops the scheme's default port, so a host matches the
// tool's host list as written: api.github.com, not api.github.com:443.
func stripDefaultPort(host, port string) string {
	if h, p, err := net.SplitHostPort(host); err == nil && p == port {
		return h
	}
	return host
}
