package egress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type service struct {
	d   Deps
	cfg Config
}

// Forward checks the placeholders before anything leaves, so a refused
// request never reaches the host. The log names the host and the decision,
// never a credential or a placeholder.
func (s *service) Forward(ctx context.Context, r Request) (Response, error) {
	log := s.d.Log.With("method", r.Method, "host", r.Host, "path", r.Path)
	res, err := s.forward(ctx, r)
	switch {
	case errors.Is(err, ErrUnauthenticated), errors.Is(err, ErrDenied), errors.Is(err, ErrTooLarge):
		log.Info("egress refused", "reason", err)
	case err != nil:
		log.Error("egress failed", "err", err)
	default:
		log.Info("egress done", "status", res.Status, "swapped", true)
	}
	return res, err
}

func (s *service) forward(ctx context.Context, r Request) (Response, error) {
	if r.Scheme != "https" && !(r.Scheme == "http" && s.cfg.AllowHTTP) {
		return Response{}, fmt.Errorf("%w: scheme %q", ErrDenied, r.Scheme)
	}
	if r.Host == "" {
		return Response{}, fmt.Errorf("%w: no host", ErrDenied)
	}
	phs := findPlaceholders(r.Header)
	if len(phs) == 0 {
		return Response{}, fmt.Errorf("%w: no placeholder in the request", ErrUnauthenticated)
	}
	real := make(map[string]string, len(phs))
	for _, ph := range phs {
		sec, err := s.d.Placeholders.Resolve(ctx, ph, r.Host)
		if err != nil {
			return Response{}, err
		}
		real[ph] = sec.Reveal()
	}
	if len(r.Body) > s.cfg.MaxBody {
		return Response{}, fmt.Errorf("%w: request body over %d bytes", ErrTooLarge, s.cfg.MaxBody)
	}

	u := url.URL{Scheme: r.Scheme, Host: r.Host, Path: r.Path, RawPath: r.RawPath, RawQuery: r.RawQuery}
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), bytes.NewReader(r.Body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	req.Header = swap(dropHopByHop(r.Header), real)
	resp, err := s.d.Upstream.Do(ctx, req)
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: %w", r.Method, r.Host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(s.cfg.MaxBody)+1))
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	if len(body) > s.cfg.MaxBody { // an error, never a success with a cut body
		return Response{}, fmt.Errorf("%w: response over %d bytes", ErrTooLarge, s.cfg.MaxBody)
	}
	body, hdr := hide(body, dropHopByHop(resp.Header), real)
	return Response{Status: resp.StatusCode, Header: hdr, Body: body}, nil
}
