package api

// ConfirmExtensionFills reports whether each password or code a linked extension fills waits for the owner to confirm it.
func (s *Service) ConfirmExtensionFills() (bool, error) {
	return s.preferences.ConfirmExtensionFills(), nil
}

// SetConfirmExtensionFills records whether fills wait for the owner, applied from the next request a linked extension makes.
func (s *Service) SetConfirmExtensionFills(confirm bool) error {
	return present(s.preferences.SetConfirmExtensionFills(confirm))
}
