//go:build android

package bridge

import "github.com/dortanes/ravenpass/packages/app/api"

var _ api.Printer = Printer{}

// Printer is Android's print service, fed from an offscreen WebView that loads nothing from the network.
type Printer struct{}

// Print shows the system print dialog for page and returns once the owner leaves it.
func (Printer) Print(job string, page []byte) error { return printError(printPage(job, page)) }
