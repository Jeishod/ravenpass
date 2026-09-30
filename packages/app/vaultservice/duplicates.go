package vaultservice

import "github.com/dortanes/ravenpass/packages/vault"

// Duplicate saves a copy of the item id named label, in its groups, and returns the copy's ID. A credential's passkeys
// stay with the original, since a copy of a private key would sign as the same passkey from two items; an identity's
// scans are copied with it, since a scan belongs to one document.
func (s *Service) Duplicate(id vault.ID, label string) (vault.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return vault.ID{}, ErrNotReady
	}
	entries, err := s.session.List()
	if err != nil {
		return vault.ID{}, err
	}
	for _, entry := range entries {
		if entry.ID != id {
			continue
		}
		pending, copied, err := s.prepareCopy(entry, label)
		if err != nil {
			return vault.ID{}, err
		}
		if err := s.commit(pending); err != nil {
			return vault.ID{}, err
		}
		return copied, nil
	}
	return vault.ID{}, vault.ErrNotFound
}

// prepareCopy prepares a copy of entry's item named label, in the entry's groups. The caller holds s.mu.
func (s *Service) prepareCopy(entry vault.Entry, label string) (*vault.Pending, vault.ID, error) {
	ticket, err := s.session.BeginSelection(entry.ID)
	if err != nil {
		return nil, vault.ID{}, err
	}
	defer s.session.ClearSelection()
	switch entry.Kind {
	case vault.KindCredential:
		credential, err := s.session.ReadSelected(ticket)
		if err != nil {
			return nil, vault.ID{}, err
		}
		for _, passkey := range credential.Passkeys {
			clear(passkey.PrivateKey)
		}
		copied := credential.CredentialInput
		copied.Label, copied.Passkeys = label, nil
		return s.session.PrepareCreate(copied, entry.Groups)
	case vault.KindCard:
		card, err := s.session.ReadSelectedCard(ticket)
		if err != nil {
			return nil, vault.ID{}, err
		}
		copied := card.CardInput
		copied.Label = label
		return s.session.PrepareCreateCard(copied, entry.Groups)
	case vault.KindNote:
		note, err := s.session.ReadSelectedNote(ticket)
		if err != nil {
			return nil, vault.ID{}, err
		}
		copied := note.NoteInput
		copied.Label = label
		return s.session.PrepareCreateNote(copied, entry.Groups)
	case vault.KindSeed:
		seed, err := s.session.ReadSelectedSeed(ticket)
		if err != nil {
			return nil, vault.ID{}, err
		}
		copied := seed.SeedInput
		copied.Label = label
		return s.session.PrepareCreateSeed(copied, entry.Groups)
	case vault.KindIdentity:
		copied, err := s.identityCopy(ticket, label)
		if err != nil {
			return nil, vault.ID{}, err
		}
		return s.session.PrepareCreateIdentity(copied, entry.Groups)
	default:
		return nil, vault.ID{}, vault.ErrNotFound
	}
}

// identityCopy is the selected identity named label, its addresses given IDs of their own and its documents attaching
// copies of their scans; a card linked to an address stays linked to the original's. The caller holds s.mu.
func (s *Service) identityCopy(ticket vault.Selection, label string) (vault.IdentityInput, error) {
	identity, err := s.session.ReadSelectedIdentity(ticket)
	if err != nil {
		return vault.IdentityInput{}, err
	}
	copied := identity.IdentityInput
	copied.Label = label
	copied.Addresses = make([]vault.Address, len(identity.Addresses))
	for i, address := range identity.Addresses {
		address.ID = vault.ID{}
		copied.Addresses[i] = address
	}
	copied.Documents = make([]vault.Document, len(identity.Documents))
	for i, document := range identity.Documents {
		for _, scan := range document.Scans {
			stored, err := s.session.ReadScan(scan)
			if err != nil {
				return vault.IdentityInput{}, err
			}
			document.Attach = append(document.Attach, stored.Copy())
		}
		document.Scans = nil
		copied.Documents[i] = document
	}
	return copied, nil
}
