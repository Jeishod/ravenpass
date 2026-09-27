package linkserver

import "github.com/dortanes/ravenpass/packages/app/linkproto"

// openSessions holds each extension ID's open session connections; the server's mu guards it.
type openSessions map[string]map[*connection]struct{}

func (o openSessions) add(id string, c *connection) {
	open, found := o[id]
	if !found {
		open = make(map[*connection]struct{})
		o[id] = open
	}
	open[c] = struct{}{}
}

func (o openSessions) remove(id string, c *connection) {
	delete(o[id], c)
	if len(o[id]) == 0 {
		delete(o, id)
	}
}

// end closes every session of id but kept with CloseNotLinked, each in its own goroutine; closing waits for the peer.
func (o openSessions) end(id string, kept *connection) {
	for c := range o[id] {
		if c != kept {
			go c.stop(linkproto.CloseNotLinked)
		}
	}
	delete(o, id)
}
