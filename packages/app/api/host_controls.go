package api

import (
	"github.com/dortanes/ravenpass/packages/app/vaultservice"
	"github.com/dortanes/ravenpass/packages/vault"
)

// HostControls are host-only calls; the web page can call every exported Service method.
type HostControls struct {
	service *Service
}

// ControlsOf returns the host-only calls of service.
func ControlsOf(service *Service) HostControls {
	return HostControls{service: service}
}

// FollowVaultFile brings the open vault up to its file, reporting true when it locked instead; that lock keeps the unlock prompt.
func (h HostControls) FollowVaultFile() bool {
	s := h.service
	if s.vault.Follow() != vaultservice.FileRefused {
		return false
	}
	s.lock()
	s.clearClipboard()
	return true
}

// LockAway closes the vault after the app left view, leaving a copy on the clipboard until its delay.
func (h HostControls) LockAway() error {
	h.service.lock()
	return nil
}

// ReceiveCodeSetup holds a usable otpauth link for codeSetupLifetime, replacing the one held; false for any other.
func (h HostControls) ReceiveCodeSetup(link string) bool {
	s := h.service
	face, err := vault.ReadSetupLink(link)
	if err != nil {
		return false
	}
	s.codeSetup.keep(link, face)
	s.vault.RecordOutsideChange()
	return true
}
