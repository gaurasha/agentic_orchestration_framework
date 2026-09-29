package tools

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var methods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// normalize fills the kind in from the spec an older client sends.
func normalize(t Tool) Tool {
	if t.Kind == "" && t.HTTP != nil {
		t.Kind = KindHTTP
	}
	if t.Kind == "" && t.Exec != nil {
		t.Kind = KindExec
	}
	return t
}

// validate checks a definition; every failure wraps ErrInvalid with the reason.
func validate(t Tool) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: empty name", ErrInvalid)
	}
	switch t.Kind {
	case KindHTTP:
		if t.HTTP == nil || t.Exec != nil {
			return fmt.Errorf("%w: an http tool needs http and no exec", ErrInvalid)
		}
		if err := validateHTTP(*t.HTTP); err != nil {
			return err
		}
		if c := t.Credential; c != nil && (c.Ref == "" || c.Header == "") {
			return fmt.Errorf("%w: an http tool's credential needs a reference and a header", ErrInvalid)
		}
	case KindExec:
		if t.Exec == nil || t.HTTP != nil {
			return fmt.Errorf("%w: an exec tool needs exec and no http", ErrInvalid)
		}
		if len(t.Exec.Argv) == 0 || t.Exec.Argv[0] == "" {
			return fmt.Errorf("%w: empty command", ErrInvalid)
		}
		if t.Exec.TimeoutSeconds < 0 {
			return fmt.Errorf("%w: negative timeout", ErrInvalid)
		}
		if c := t.Credential; c != nil {
			if c.Ref == "" || !envName.MatchString(c.Env) {
				return fmt.Errorf("%w: an exec tool's credential needs a reference and an environment variable name", ErrInvalid)
			}
			if len(c.Hosts) == 0 {
				return fmt.Errorf("%w: an exec tool's credential needs the hosts it may reach", ErrInvalid)
			}
			for _, h := range c.Hosts {
				if h == "" || strings.ContainsAny(h, "/{ ") {
					return fmt.Errorf("%w: host %q", ErrInvalid, h)
				}
			}
		}
	default:
		return fmt.Errorf("%w: kind must be http or exec", ErrInvalid)
	}
	return nil
}

func validateHTTP(h HTTP) error {
	if !methods[h.Method] {
		return fmt.Errorf("%w: method %q", ErrInvalid, h.Method)
	}
	u, err := url.Parse(h.URL)
	if err != nil {
		return fmt.Errorf("%w: url: %v", ErrInvalid, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: url scheme must be http or https", ErrInvalid)
	}
	if u.Host == "" || strings.Contains(u.Host, "{") || u.User != nil {
		return fmt.Errorf("%w: url host must be fixed, with no placeholder", ErrInvalid)
	}
	return nil
}
