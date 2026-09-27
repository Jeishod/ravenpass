package api

import "github.com/dortanes/ravenpass/packages/app/preferences"

// InterfaceSize is the chosen interface size and the sizes offered, in percent.
type InterfaceSize struct {
	Percent int   `json:"percent"`
	Offered []int `json:"offered"`
}

// GetInterfaceSize reports the chosen interface size and the sizes offered.
func (s *Service) GetInterfaceSize() (InterfaceSize, error) {
	return InterfaceSize{Percent: s.preferences.InterfaceSize(), Offered: preferences.InterfaceSizes()}, nil
}

// SetInterfaceSize records one of the offered sizes; the interface applies it.
func (s *Service) SetInterfaceSize(percent int) error {
	return present(s.preferences.SetInterfaceSize(percent))
}
