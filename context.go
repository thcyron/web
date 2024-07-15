package web

import "context"

type contextKey string

const (
	contextKeySite contextKey = "site"
)

func SiteFromContext(ctx context.Context) *Site {
	return ctx.Value(contextKeySite).(*Site)
}

func contextWithSite(ctx context.Context, site *Site) context.Context {
	return context.WithValue(ctx, contextKeySite, site)
}

func Asset(ctx context.Context, name string) string {
	return SiteFromContext(ctx).Asset(name)
}
