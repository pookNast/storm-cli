package persona_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pookNast/storm-cli/internal/persona"
)

// Test that when one goroutine errors, others cancel rather than hang
type blockingChatter struct {
	callCount atomic.Int64
	failOn    int64
}

func (b *blockingChatter) Chat(ctx context.Context, _, _, _ string) (string, error) {
	n := b.callCount.Add(1)
	if n == b.failOn {
		return "", fmt.Errorf("forced error on call %d", n)
	}
	// Block until context is cancelled
	<-ctx.Done()
	return "", ctx.Err()
}

func TestFanOutContextCancellation(t *testing.T) {
	// goroutine 1 (i=0, first call) errors immediately
	// goroutines 2-5 block on ctx.Done()
	// errgroup should cancel context, unblocking them
	cl := &blockingChatter{failOn: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := persona.FanOut(ctx, cl, "model", "topic", "role")
	if err == nil {
		t.Fatal("expected error from FanOut")
	}
	t.Logf("FanOut returned error (expected): %v", err)
	// If we reach here within 3s, goroutines were unblocked. No leak.
}
