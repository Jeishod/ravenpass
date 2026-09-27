package api

import "errors"

// SystemAutofill is the device's choice, made on a system screen, of autofill service and passkey providers.
type SystemAutofill interface {
	// Status reports what the device has chosen Ravenpass for.
	Status() SystemAutofillStatus
	// Choose shows the system screen where the owner chooses Ravenpass and returns once they leave it.
	Choose() error
}

// SystemAutofillStatus is what the device has chosen Ravenpass for.
type SystemAutofillStatus struct {
	// Autofill reports that Ravenpass is the device's autofill service.
	Autofill bool `json:"autofill"`
	// Passkeys reports that Ravenpass is one of the device's passkey providers.
	Passkeys bool `json:"passkeys"`
	// PasskeyProviders reports that the device takes passkey providers; when false, Passkeys is false too.
	PasskeyProviders bool `json:"passkeyProviders"`
}

var errNoSystemAutofill = errors.New("the host has no system autofill to choose")

// noSystemAutofill is the choice of a host whose system fills in nothing for Ravenpass.
type noSystemAutofill struct{}

func (noSystemAutofill) Status() SystemAutofillStatus { return SystemAutofillStatus{} }

func (noSystemAutofill) Choose() error { return errNoSystemAutofill }

// GetSystemAutofill reports whether Ravenpass is the device's autofill service and a passkey provider.
func (s *Service) GetSystemAutofill() (SystemAutofillStatus, error) {
	return s.systemAutofill.Status(), nil
}

// ChooseSystemAutofill shows the system autofill screen, holding the host until the owner leaves it.
func (s *Service) ChooseSystemAutofill() error {
	defer s.hold()()
	return present(s.systemAutofill.Choose())
}
