package preferences

import (
	"errors"
	"slices"
)

// defaultInterfaceSize is the interface at the host's own scale, in percent.
const defaultInterfaceSize = 100

var interfaceSizes = []int{85, 100, 115, 130, 150}

// ErrUnsupportedInterfaceSize reports a size this build does not offer.
var ErrUnsupportedInterfaceSize = errors.New("unsupported interface size")

// InterfaceSizes lists the sizes the user may choose, in percent, smallest first.
func InterfaceSizes() []int { return slices.Clone(interfaceSizes) }

// InterfaceSize reports the chosen size in percent, or the default where none was recorded.
func (s *Store) InterfaceSize() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if !slices.Contains(interfaceSizes, s.current.InterfaceSize) {
		return defaultInterfaceSize
	}
	return s.current.InterfaceSize
}

// SetInterfaceSize records one of the offered sizes.
func (s *Store) SetInterfaceSize(percent int) error {
	if !slices.Contains(interfaceSizes, percent) {
		return ErrUnsupportedInterfaceSize
	}
	return s.update(func(next *record) { next.InterfaceSize = percent })
}
