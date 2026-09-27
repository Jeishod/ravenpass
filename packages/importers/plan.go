package importers

import (
	"slices"
	"strings"

	"github.com/dortanes/ravenpass/packages/vault"
)

// Options are the choices a person makes before importing; a card that names no network takes its issuer's CardNetworks entry.
type Options struct {
	Groups         bool // folders and collections become groups
	SkipDuplicates bool
	CardNetworks   map[string]vault.CardNetwork // by issuer identification number
}

// Conversion counts the items of one Ravenpass kind that came from another kind of the source.
type Conversion struct {
	From  Origin
	Count int
}

// KindPreview counts what the import adds of one kind, duplicates included, and which of those the vault already holds.
type KindPreview struct {
	Kind         vault.Kind
	Count        int
	Duplicates   int
	OneTimeCodes int
	Converted    []Conversion
}

// GroupPreview is what the folder and collection names of the imported items come to.
type GroupPreview struct {
	New      int // groups created
	Existing int // folder names matching a group the vault holds
	Dropped  int // folder names that cannot become groups
}

// Preview is what an import adds and leaves behind, known before anything is written.
type Preview struct {
	Format                  Format
	Items                   int           // items the file holds
	Kinds                   []KindPreview // in previewOrder, kinds with no item left out
	Groups                  GroupPreview
	GroupsWithoutDuplicates GroupPreview
	// CardIssuers are the distinct issuers of the cards that name no network, in file order.
	CardIssuers []string
	Skipped     []Skip
	Attachments int
	// Passkeys counts those left behind, ImportedPasskeys those brought over, duplicates included.
	Passkeys         int
	ImportedPasskeys int
}

// Plan is an export laid against an open vault; it keeps its own copies of what it was given.
type Plan struct {
	preview           Preview
	items             []planned
	withDuplicates    filing
	withoutDuplicates filing
}

// planned is an item the vault accepts; duplicate reports an entry the vault holds that lists the same.
type planned struct {
	content   vault.NewItem
	kind      vault.Kind
	origin    Origin
	folders   []string
	duplicate bool
}

// filing is the group names each planned item joins, nil for one left out, with the counts.
type filing struct {
	names    [][]string
	created  int
	existing int
	dropped  int
}

// previewOrder is the order Preview lists kinds in.
var previewOrder = []vault.Kind{vault.KindCredential, vault.KindCard, vault.KindIdentity, vault.KindNote, vault.KindSeed}

// sameKind is the source kind each Ravenpass kind comes from without a conversion; a seed has none.
var sameKind = map[vault.Kind]Origin{
	vault.KindCredential: OriginLogin,
	vault.KindCard:       OriginCard,
	vault.KindIdentity:   OriginIdentity,
	vault.KindNote:       OriginNote,
}

// NewPlan lays an export against what an open vault lists; an item the vault refuses is skipped as too long.
func NewPlan(export Export, entries []vault.Entry, groups []vault.Group) *Plan {
	held := make(map[listing]struct{}, len(entries))
	for _, entry := range entries {
		if listed, comparable := listingOf(entry); comparable {
			held[listed] = struct{}{}
		}
	}
	plan := &Plan{preview: Preview{
		Format:      export.Format,
		Items:       len(export.Items) + len(export.Skipped),
		Skipped:     append([]Skip{}, export.Skipped...),
		Attachments: export.Attachments,
		Passkeys:    export.Passkeys,
	}}
	for _, item := range export.Items {
		content := contentOf(item.Content)
		entry, err := vault.PreviewNewItem(content)
		passkeys := len(passkeysOf(content))
		if err != nil {
			plan.preview.Skipped = append(plan.preview.Skipped, Skip{Label: labelOf(content), Origin: item.Origin, Reason: ReasonTooLong})
			plan.preview.Passkeys += passkeys
			continue
		}
		duplicate := false
		if listed, comparable := listingOf(entry); comparable {
			_, duplicate = held[listed]
		}
		plan.items = append(plan.items, planned{content: content, kind: entry.Kind, origin: item.Origin, folders: slices.Clone(item.Folders), duplicate: duplicate})
		plan.preview.ImportedPasskeys += passkeys
	}
	plan.preview.Kinds = previewKinds(plan.items)
	plan.preview.CardIssuers = cardIssuers(plan.items)
	plan.withDuplicates = fileFolders(plan.items, groups, false)
	plan.withoutDuplicates = fileFolders(plan.items, groups, true)
	plan.preview.Groups = plan.withDuplicates.preview()
	plan.preview.GroupsWithoutDuplicates = plan.withoutDuplicates.preview()
	return plan
}

// Preview reports what the import adds, what the vault already holds and what stays behind.
func (p *Plan) Preview() Preview {
	preview := p.preview
	preview.CardIssuers = append([]string{}, p.preview.CardIssuers...)
	preview.Skipped = append([]Skip{}, p.preview.Skipped...)
	preview.Kinds = make([]KindPreview, len(p.preview.Kinds))
	for i, kind := range p.preview.Kinds {
		kind.Converted = append([]Conversion{}, kind.Converted...)
		preview.Kinds[i] = kind
	}
	return preview
}

// Items returns copies of the accepted items in file order, as options choose.
func (p *Plan) Items(options Options) []vault.NewItem {
	filed := p.withDuplicates
	if options.SkipDuplicates {
		filed = p.withoutDuplicates
	}
	items := make([]vault.NewItem, 0, len(p.items))
	for i, item := range p.items {
		if options.SkipDuplicates && item.duplicate {
			continue
		}
		content := contentOf(item.content)
		if options.Groups {
			content.Groups = slices.Clone(filed.names[i])
		}
		if card := content.Card; card != nil && card.Network == 0 {
			card.Network = options.CardNetworks[cardIssuer(card.Number)]
		}
		items = append(items, content)
	}
	return items
}

// cardIssuers lists the distinct issuers of the cards that name no network, in file order.
func cardIssuers(items []planned) []string {
	issuers := []string{}
	for _, item := range items {
		if card := item.content.Card; card != nil && card.Network == 0 {
			if issuer := cardIssuer(card.Number); !slices.Contains(issuers, issuer) {
				issuers = append(issuers, issuer)
			}
		}
	}
	return issuers
}

// listing is what the duplicate rule compares of an entry.
type listing struct {
	kind      vault.Kind
	label     string
	detail    string
	site      string
	email     string
	lastFour  string
	expiresOn string
	seed      vault.SeedFace
}

// listingOf reports false for a hidden note, which lists its label alone.
func listingOf(entry vault.Entry) (listing, bool) {
	listed := listing{kind: entry.Kind, label: strings.ToLower(strings.TrimSpace(entry.Label))}
	switch entry.Kind {
	case vault.KindCredential:
		listed.detail, listed.site, listed.email = entry.Detail, entry.Site, entry.Email
	case vault.KindCard:
		listed.lastFour, listed.expiresOn = entry.Card.LastFour, entry.ExpiresOn
	case vault.KindIdentity:
		listed.detail, listed.expiresOn = entry.Detail, entry.ExpiresOn
	case vault.KindNote:
		if entry.Note.Hidden {
			return listing{}, false
		}
		listed.detail = entry.Detail
	case vault.KindSeed:
		listed.detail, listed.seed = entry.Detail, entry.Seed
	}
	return listed, true
}

func previewKinds(items []planned) []KindPreview {
	kinds := []KindPreview{}
	for _, kind := range previewOrder {
		preview := KindPreview{Kind: kind, Converted: []Conversion{}}
		for _, item := range items {
			if item.kind != kind {
				continue
			}
			preview.Count++
			if item.duplicate {
				preview.Duplicates++
			}
			if item.content.Credential != nil && item.content.Credential.TOTP != "" {
				preview.OneTimeCodes++
			}
			if item.origin != sameKind[kind] {
				preview.Converted = counted(preview.Converted, item.origin)
			}
		}
		if preview.Count > 0 {
			kinds = append(kinds, preview)
		}
	}
	return kinds
}

// counted adds one item from origin, keeping origins in the order first counted.
func counted(conversions []Conversion, origin Origin) []Conversion {
	for i := range conversions {
		if conversions[i].From == origin {
			conversions[i].Count++
			return conversions
		}
	}
	return append(conversions, Conversion{From: origin, Count: 1})
}

// fileFolders turns folder names into group names, dropping refused names and new ones past vault.MaxGroups; a new group takes its first spelling.
func fileFolders(items []planned, groups []vault.Group, skipDuplicates bool) filing {
	held := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		held[vault.GroupKey(group.Name)] = struct{}{}
	}
	room := max(vault.MaxGroups-len(groups), 0)
	created := make(map[string]string)
	named := make(map[string]struct{})
	joined := make(map[string]struct{})
	filed := filing{names: make([][]string, len(items))}
	for i, item := range items {
		if skipDuplicates && item.duplicate {
			continue
		}
		var names []string
		for _, folder := range item.folders {
			key := vault.GroupKey(folder)
			named[key] = struct{}{}
			name, err := vault.AcceptGroupName(folder)
			if err != nil || len(names) == vault.MaxCredentialGroups || slices.ContainsFunc(names, func(taken string) bool { return vault.GroupKey(taken) == key }) {
				continue
			}
			if _, isHeld := held[key]; !isHeld {
				first, isCreated := created[key]
				switch {
				case isCreated:
					name = first
				case len(created) < room:
					created[key] = name
				default:
					continue
				}
			}
			joined[key] = struct{}{}
			names = append(names, name)
		}
		filed.names[i] = names
	}
	filed.created = len(created)
	for key := range joined {
		if _, isHeld := held[key]; isHeld {
			filed.existing++
		}
	}
	filed.dropped = len(named) - len(joined)
	return filed
}

func (f filing) preview() GroupPreview {
	return GroupPreview{New: f.created, Existing: f.existing, Dropped: f.dropped}
}

func labelOf(item vault.NewItem) string {
	switch {
	case item.Credential != nil:
		return item.Credential.Label
	case item.Card != nil:
		return item.Card.Label
	case item.Identity != nil:
		return item.Identity.Label
	case item.Note != nil:
		return item.Note.Label
	case item.Seed != nil:
		return item.Seed.Label
	}
	return ""
}

// passkeysOf are the passkeys of a credential; nil for any other kind.
func passkeysOf(item vault.NewItem) []vault.Passkey {
	if item.Credential == nil {
		return nil
	}
	return item.Credential.Passkeys
}

// contentOf deep-copies an item's content and pin, without group names; a batch item holds no card link, photo or scan.
func contentOf(item vault.NewItem) vault.NewItem {
	clone := vault.NewItem{Pinned: item.Pinned}
	if item.Credential != nil {
		credential := *item.Credential
		credential.Websites = slices.Clone(credential.Websites)
		credential.Apps = slices.Clone(credential.Apps)
		credential.Passkeys = slices.Clone(credential.Passkeys)
		for i := range credential.Passkeys {
			passkey := &credential.Passkeys[i]
			passkey.CredentialID = slices.Clone(passkey.CredentialID)
			passkey.UserHandle = slices.Clone(passkey.UserHandle)
			passkey.PrivateKey = slices.Clone(passkey.PrivateKey)
		}
		clone.Credential = &credential
	}
	if item.Card != nil {
		card := *item.Card
		if card.Billing != nil {
			billing := *card.Billing
			card.Billing = &billing
		}
		clone.Card = &card
	}
	if item.Identity != nil {
		identity := *item.Identity
		identity.Emails = slices.Clone(identity.Emails)
		identity.Phones = slices.Clone(identity.Phones)
		identity.Addresses = slices.Clone(identity.Addresses)
		identity.Documents = slices.Clone(identity.Documents)
		clone.Identity = &identity
	}
	if item.Note != nil {
		note := *item.Note
		clone.Note = &note
	}
	if item.Seed != nil {
		seed := *item.Seed
		seed.Words = slices.Clone(seed.Words)
		seed.Codes = slices.Clone(seed.Codes)
		seed.Addresses = slices.Clone(seed.Addresses)
		clone.Seed = &seed
	}
	return clone
}
