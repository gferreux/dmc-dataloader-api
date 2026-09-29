package port

import "context"

type principalContextKey struct{}

// WithPrincipal stores the authenticated caller on the context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFrom returns the caller stored by WithPrincipal.
func PrincipalFrom(ctx context.Context) Principal {
	principal, _ := ctx.Value(principalContextKey{}).(Principal)

	return principal
}
