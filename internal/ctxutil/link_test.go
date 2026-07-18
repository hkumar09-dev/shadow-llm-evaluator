package ctxutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/ctxutil"
)

func TestLink_cancelsWhenParentCancels(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	link := context.Background()

	ctx, cancel := ctxutil.Link(parent, link)
	defer cancel()

	parentCancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected ctx canceled when parent cancels")
	}
}

func TestLink_cancelsWhenLinkCancels(t *testing.T) {
	parent := context.Background()
	link, linkCancel := context.WithCancel(context.Background())

	ctx, cancel := ctxutil.Link(parent, link)
	defer cancel()

	linkCancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected ctx canceled when link cancels")
	}
}
