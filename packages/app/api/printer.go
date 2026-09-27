package api

import "errors"

// Printer shows the system print dialog for one page.
type Printer interface {
	// Print shows the dialog for page, a UTF-8 HTML document that loads nothing and runs no script, titled job, and
	// returns once the owner prints or cancels. The page stays in memory; the caller clears it after Print returns.
	Print(job string, page []byte) error
}

var errNoPrinter = errors.New("this host has no print dialog")

type noPrinter struct{}

func (noPrinter) Print(string, []byte) error { return errNoPrinter }
