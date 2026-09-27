// Package changecount counts changes that a waiter learns of once the count passes the one it saw.
package changecount

import (
	"context"
	"sync"
)

// Counter counts changes since Ravenpass started; its zero value is ready and a wait never holds its owner's lock.
type Counter struct {
	mu      sync.Mutex
	count   uint64
	changed chan struct{}
}

// Record counts one change and wakes every wait.
func (c *Counter) Record() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	if c.changed != nil {
		close(c.changed)
		c.changed = nil
	}
}

// Await returns the count once it passes seen. It fails with ctx's error when ctx ends first.
func (c *Counter) Await(ctx context.Context, seen uint64) (uint64, error) {
	for {
		c.mu.Lock()
		if c.count > seen {
			count := c.count
			c.mu.Unlock()
			return count, nil
		}
		if c.changed == nil {
			c.changed = make(chan struct{})
		}
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}
