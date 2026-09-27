package api

// IdentityList is the system's list of the vault's accounts it suggests in its own autofill.
type IdentityList interface {
	// Changed brings the list in step with the recorded choice without waiting.
	Changed()
}

// noIdentityList is the list of a host whose system keeps none.
type noIdentityList struct{}

func (noIdentityList) Changed() {}

// IdentityListSetting reports whether the system keeps the vault's websites and user names for its autofill.
type IdentityListSetting struct {
	Enabled bool `json:"enabled"`
}

// GetIdentityList reports whether the system keeps the vault's accounts.
func (s *Service) GetIdentityList() (IdentityListSetting, error) {
	return IdentityListSetting{Enabled: s.preferences.IdentityList()}, nil
}

// SetIdentityList records the choice; when on, the system keeps the accounts outside the vault's encryption.
func (s *Service) SetIdentityList(enabled bool) error {
	if err := s.preferences.SetIdentityList(enabled); err != nil {
		return present(err)
	}
	s.identities.Changed()
	return nil
}
