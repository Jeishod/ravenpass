package vault

import "slices"

// NewItem is one item a batch adds, with exactly one content set and no card link, photo or scan; Groups match by GroupKey.
type NewItem struct {
	Credential *CredentialInput
	Card       *CardInput
	Identity   *IdentityInput
	Note       *NoteInput
	Seed       *SeedInput
	Pinned     bool
	Groups     []string
}

// Kind is the kind of the item's content, or zero when it holds none or more than one.
func (item NewItem) Kind() Kind {
	var kind Kind
	for _, content := range []struct {
		set  bool
		kind Kind
	}{
		{item.Credential != nil, KindCredential},
		{item.Card != nil, KindCard},
		{item.Identity != nil, KindIdentity},
		{item.Note != nil, KindNote},
		{item.Seed != nil, KindSeed},
	} {
		if !content.set {
			continue
		}
		if kind != 0 {
			return 0
		}
		kind = content.kind
	}
	return kind
}

// BatchResult names the items a batch added, in the order given, and how many groups it created.
type BatchResult struct {
	Items  []ID
	Groups int
}

// acceptedItem is a batch item as stored; entry carries no membership and groups hold one name per group.
type acceptedItem struct {
	content NewItem
	entry   entryMeta
	groups  []string
}

// PreviewNewItem reports, without a session, the entry an item would list as, with a zero ID and no groups.
func PreviewNewItem(item NewItem) (Entry, error) {
	accepted, err := acceptNewItem(item)
	if err != nil {
		return Entry{}, err
	}
	return accepted.entry.listed(), nil
}

func acceptNewItem(item NewItem) (acceptedItem, error) {
	var accepted acceptedItem
	switch item.Kind() {
	case KindCredential:
		input, err := acceptInput(*item.Credential)
		if err != nil {
			return acceptedItem{}, err
		}
		accepted.content.Credential = &input
		accepted.entry = credentialEntry(input, nil)
	case KindCard:
		if item.Card.BillingLink != nil {
			return acceptedItem{}, ErrInvalidInput
		}
		input, err := acceptCard(*item.Card)
		if err != nil {
			return acceptedItem{}, err
		}
		accepted.content.Card = &input
		accepted.entry = cardEntry(input, nil)
	case KindIdentity:
		if !bareIdentity(*item.Identity) {
			return acceptedItem{}, ErrInvalidInput
		}
		input, err := acceptIdentity(*item.Identity)
		if err != nil {
			return acceptedItem{}, err
		}
		accepted.content.Identity = &input
		accepted.entry = identityEntry(input, nil, nil)
	case KindNote:
		input, err := acceptNote(*item.Note)
		if err != nil {
			return acceptedItem{}, err
		}
		accepted.content.Note = &input
		accepted.entry = noteEntry(input, nil)
	case KindSeed:
		input, err := acceptSeed(*item.Seed)
		if err != nil {
			return acceptedItem{}, err
		}
		accepted.content.Seed = &input
		accepted.entry = seedEntry(input, nil)
	default:
		return acceptedItem{}, ErrInvalidInput
	}
	groups, err := acceptGroupNames(item.Groups)
	if err != nil {
		return acceptedItem{}, err
	}
	accepted.content.Pinned = item.Pinned
	accepted.entry.pinned = item.Pinned
	accepted.groups = groups
	return accepted, nil
}

// bareIdentity reports whether a batch may add the identity.
func bareIdentity(input IdentityInput) bool {
	if len(input.Photo) > 0 {
		return false
	}
	for _, address := range input.Addresses {
		if address.ID != (ID{}) {
			return false
		}
	}
	for _, document := range input.Documents {
		if len(document.Scans) > 0 || len(document.Attach) > 0 {
			return false
		}
	}
	return true
}

// acceptGroupNames accepts the names an item joins, keeping the first spelling of each group.
func acceptGroupNames(names []string) ([]string, error) {
	accepted := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name, err := AcceptGroupName(name)
		if err != nil {
			return nil, err
		}
		key := GroupKey(name)
		if _, repeated := seen[key]; repeated {
			continue
		}
		seen[key] = struct{}{}
		accepted = append(accepted, name)
	}
	if len(accepted) > MaxCredentialGroups {
		return nil, ErrResourceLimit
	}
	return accepted, nil
}

// record encodes the item's content; an identity's addresses receive their IDs here.
func (a acceptedItem) record() ([]byte, error) {
	switch {
	case a.content.Credential != nil:
		return encodeCredentialRecord(*a.content.Credential)
	case a.content.Card != nil:
		return encodeCardRecord(*a.content.Card)
	case a.content.Identity != nil:
		input := *a.content.Identity
		input.Addresses = slices.Clone(input.Addresses)
		if err := assignAddressIDs(input.Addresses, nil); err != nil {
			return nil, err
		}
		return encodeIdentityRecord(input)
	case a.content.Seed != nil:
		return encodeSeedRecord(*a.content.Seed, "")
	default:
		return encodeNoteRecord(*a.content.Note)
	}
}

// PrepareAddItems prepares one save that adds every item and the groups they name, or nothing when any item is refused.
func (s *Session) PrepareAddItems(items []NewItem) (*Pending, BatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyToWrite(); err != nil {
		return nil, BatchResult{}, err
	}
	if len(items) == 0 {
		return nil, BatchResult{}, ErrInvalidInput
	}
	accepted := make([]acceptedItem, len(items))
	for i, item := range items {
		var err error
		if accepted[i], err = acceptNewItem(item); err != nil {
			return nil, BatchResult{}, err
		}
	}
	groups, memberships, created, err := s.joinGroups(accepted)
	if err != nil {
		return nil, BatchResult{}, err
	}
	entries := append([]entryMeta(nil), s.entries...)
	records := append([]sealedBox(nil), s.records...)
	reserved := make(map[ID]struct{}, len(accepted))
	result := BatchResult{Items: make([]ID, 0, len(accepted)), Groups: created}
	for i, item := range accepted {
		id, err := s.freshID(reserved)
		if err != nil {
			return nil, BatchResult{}, err
		}
		reserved[id] = struct{}{}
		plaintext, err := item.record()
		if err != nil {
			return nil, BatchResult{}, err
		}
		listed := item.entry
		listed.groups = memberships[i]
		entry, box, err := s.sealFirst(id, listed, plaintext)
		clear(plaintext)
		if err != nil {
			return nil, BatchResult{}, err
		}
		entries = append(entries, entry)
		records = append(records, box)
		result.Items = append(result.Items, id)
	}
	pending, err := s.prepareWithGroups(entries, records, groups)
	if err != nil {
		return nil, BatchResult{}, err
	}
	return pending, result, nil
}

// joinGroups returns the resulting groups, each item's membership and the count of new groups, created in first-named order.
func (s *Session) joinGroups(items []acceptedItem) ([]Group, [][]ID, int, error) {
	groups := append([]Group(nil), s.groups...)
	byKey := make(map[string]ID, len(groups))
	for _, group := range groups {
		byKey[GroupKey(group.Name)] = group.ID
	}
	created := 0
	memberships := make([][]ID, len(items))
	for i, item := range items {
		ids := make([]ID, 0, len(item.groups))
		for _, name := range item.groups {
			key := GroupKey(name)
			id, held := byKey[key]
			if !held {
				if len(groups) >= MaxGroups {
					return nil, nil, 0, ErrResourceLimit
				}
				var err error
				id, err = uniqueID(func(candidate ID) bool {
					return slices.ContainsFunc(groups, func(group Group) bool { return group.ID == candidate })
				})
				if err != nil {
					return nil, nil, 0, err
				}
				groups = append(groups, Group{ID: id, Name: name})
				byKey[key] = id
				created++
			}
			ids = append(ids, id)
		}
		membership, err := acceptMembership(ids, groups)
		if err != nil {
			return nil, nil, 0, err
		}
		memberships[i] = membership
	}
	return groups, memberships, created, nil
}
