package changecount

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAWaitReturnsOnceTheCountPassesWhatItSaw(t *testing.T) {
	var counter Counter
	counted := make(chan uint64, 1)
	go func() {
		count, err := counter.Await(context.Background(), 0)
		if err == nil {
			counted <- count
		}
	}()
	counter.Record()
	select {
	case count := <-counted:
		if count != 1 {
			t.Fatalf("count = %d, want 1", count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a wait did not see the change")
	}
	counter.Record()
	if count, err := counter.Await(context.Background(), 1); err != nil || count != 2 {
		t.Fatalf("a wait behind the count = %d, %v", count, err)
	}
}

func TestAWaitEndsWithItsContext(t *testing.T) {
	var counter Counter
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := counter.Await(ctx, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a wait with no change: got %v, want DeadlineExceeded", err)
	}
}
