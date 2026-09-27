package extensionaccess

import (
	"sync"
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// confirmedFillLifetime matches how long the extension keeps a sign-in's credential for the form's password step.
const confirmedFillLifetime = time.Minute

type confirmedFill struct {
	extension  string
	credential vault.ID
	origin     string
}

// confirmedFills spares one second confirmation of a fill the person just confirmed: the extension already holds the
// credential's values, so filling it again for the same extension and page releases nothing new.
type confirmedFills struct {
	now func() time.Time

	mu      sync.Mutex
	expires map[confirmedFill]time.Time
}

func newConfirmedFills(now func() time.Time) *confirmedFills {
	return &confirmedFills{now: now, expires: map[confirmedFill]time.Time{}}
}

func (c *confirmedFills) keep(fill confirmedFill) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for kept, expires := range c.expires {
		if !now.Before(expires) {
			delete(c.expires, kept)
		}
	}
	c.expires[fill] = now.Add(confirmedFillLifetime)
}

// take reports whether fill was confirmed within confirmedFillLifetime, and spares it only once.
func (c *confirmedFills) take(fill confirmedFill) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	expires, found := c.expires[fill]
	delete(c.expires, fill)
	return found && c.now().Before(expires)
}
