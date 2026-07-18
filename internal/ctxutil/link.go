// Package ctxutil helpers for composing request and application contexts.
package ctxutil

import "context"

// Link returns a child of parent that is also canceled when link is canceled.
// Caller must call the returned CancelFunc (typically via defer).
//
// Typical use in HTTP handlers:
//
//	ctx, cancel := ctxutil.Link(c.Request.Context(), appCtx)
//	defer cancel()
//
// so work stops on client disconnect OR process shutdown.
func Link(parent, link context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	if link == nil {
		return ctx, cancel
	}
	stop := context.AfterFunc(link, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
