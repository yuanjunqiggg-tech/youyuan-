package link_connection

import (
	"context"
	"net"
)

type linkDialerContextKey struct{}

// WithDialer binds an outbound TCP transport to this request only. Derived
// contexts retain it; other users and clients keep their own/native transport.
// The injected dialer must respect ctx and return a regular net.Conn.
func WithDialer(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error)) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if dial == nil {
		return ctx
	}
	return context.WithValue(ctx, linkDialerContextKey{}, dial)
}
