package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type service struct{ d Deps }

func (s *service) Issue(_ context.Context, tenant string, ttl time.Duration) (string, error) {
	if tenant == "" {
		return "", fmt.Errorf("%w: empty tenant", ErrInvalid)
	}
	return sign(Claims{Tenant: tenant, Expires: s.d.Clock.Now().Add(ttl)}, []byte(s.d.Key.Reveal()))
}

func (s *service) Verify(_ context.Context, token string) (Claims, error) {
	return parse(token, []byte(s.d.Key.Reveal()), s.d.Clock.Now())
}

func (s *service) Resolve(ctx context.Context, tenant, ref string) (Secret, error) {
	sec, err := s.d.Secrets.Get(ctx, tenant, ref)
	if err != nil {
		return Secret{}, fmt.Errorf("resolve %s: %w", ref, err)
	}
	return sec, nil
}

func (s *service) Store(ctx context.Context, tenant, ref string, sec Secret) error {
	if err := validateStore(ref, sec); err != nil {
		return err
	}
	if err := s.d.Secrets.Put(ctx, tenant, ref, sec); err != nil {
		return fmt.Errorf("store %s: %w", ref, err)
	}
	return nil
}

func (s *service) List(ctx context.Context, tenant string) ([]string, error) {
	return s.d.Secrets.List(ctx, tenant)
}

func (s *service) IssuePlaceholder(ctx context.Context, p Placeholder) (string, error) {
	if err := validatePlaceholder(p); err != nil {
		return "", err
	}
	// The credential must exist now, so a bad reference fails the call, not
	// the sandbox's request later.
	if _, err := s.d.Secrets.Get(ctx, p.Tenant, p.Ref); err != nil {
		return "", fmt.Errorf("placeholder for %s: %w", p.Ref, err)
	}
	value := PlaceholderPrefix + s.d.Random.Token()
	if err := s.d.Placeholders.Put(ctx, value, p); err != nil {
		return "", fmt.Errorf("issue placeholder: %w", err)
	}
	return value, nil
}

func (s *service) ResolvePlaceholder(ctx context.Context, value, host string) (Secret, error) {
	p, err := s.d.Placeholders.Get(ctx, value)
	if errors.Is(err, ErrUnauthenticated) {
		return Secret{}, fmt.Errorf("%w: unknown placeholder", ErrUnauthenticated)
	}
	if err != nil {
		return Secret{}, err
	}
	if err := allows(p, host, s.d.Clock.Now()); err != nil {
		return Secret{}, err
	}
	sec, err := s.d.Secrets.Get(ctx, p.Tenant, p.Ref)
	if err != nil {
		return Secret{}, fmt.Errorf("placeholder for %s: %w", p.Ref, err)
	}
	return sec, nil
}

func (s *service) RetirePlaceholder(ctx context.Context, value string) error {
	return s.d.Placeholders.Delete(ctx, value)
}
