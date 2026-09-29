package main

// The UI's JWT carries the tenant ID; every API request is scoped to that
// tenant. Sign-in is out of scope: an endpoint mints a token for a tenant.

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// tenantTokens mints and verifies the UI's JWT.
type tenantTokens interface {
	Mint(ctx context.Context, tenant string, ttl time.Duration) (string, error)
	// Verify returns the token's tenant, or an error if it is missing,
	// forged or expired.
	Verify(ctx context.Context, token string) (tenant string, err error)
}

type tenantKey struct{}

// requireTenant refuses a request without a valid bearer token and puts
// the token's tenant in the request's context.
func requireTenant(t tenantTokens, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		tenant, err := t.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tenantKey{}, tenant)))
	})
}

// tenantOf returns the tenant requireTenant stored, and whether it did, so
// a handler outside the middleware can never act as tenant "".
func tenantOf(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(tenantKey{}).(string)
	return s, ok && s != ""
}
