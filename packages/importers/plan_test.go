package importers

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dortanes/ravenpass/packages/vault"
)

func noteItem(label, body string, folders ...string) Item {
	return Item{Content: vault.NewItem{Note: &vault.NoteInput{Label: label, Body: body}}, Origin: OriginNote, Folders: folders}
}

func credentialItem(input vault.CredentialInput) Item {
	return Item{Content: vault.NewItem{Credential: &input}, Origin: OriginLogin}
}

func cardItem(label, number string) Item {
	return Item{Content: vault.NewItem{Card: &vault.CardInput{Label: label, Number: number}}, Origin: OriginCard}
}

func identityItem(label string, emails ...string) Item {
	return Item{Content: vault.NewItem{Identity: &vault.IdentityInput{Label: label, Emails: emails}}, Origin: OriginIdentity}
}

func entriesOf(t *testing.T, items ...Item) []vault.Entry {
	t.Helper()
	entries := make([]vault.Entry, len(items))
	for i, item := range items {
		entry, err := vault.PreviewNewItem(item.Content)
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = entry
	}
	return entries
}

func heldGroups(names ...string) []vault.Group {
	groups := make([]vault.Group, len(names))
	for i, name := range names {
		groups[i] = vault.Group{ID: vault.ID{byte(i + 1)}, Name: name}
	}
	return groups
}

func numberedNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("%s %d", prefix, i)
	}
	return names
}

func labelsOf(items []vault.NewItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = labelOf(item)
	}
	return labels
}

func groupsOf(items []vault.NewItem) [][]string {
	groups := make([][]string, len(items))
	for i, item := range items {
		groups[i] = item.Groups
	}
	return groups
}

func TestNewPlanFindsDuplicatesOfTheSameKind(t *testing.T) {
	mail := vault.CredentialInput{Label: "Mail", Websites: []string{"https://mail.example.test"}, Login: "alex", Email: "alex@example.test", Password: "one"}
	entries := entriesOf(t,
		credentialItem(mail),
		cardItem("Visa", "4111111111111111"),
		identityItem("Me", "me@example.test"),
		noteItem("Router", "admin\nsecond"),
	)
	relabelled := mail
	relabelled.Label, relabelled.Password = "  MAIL ", "changed"
	elsewhere := mail
	elsewhere.Websites = []string{"https://www.mail.example.test/login"}
	otherLogin := mail
	otherLogin.Login = "alexa"
	otherEmail := mail
	otherEmail.Email = "other@example.test"
	export := Export{Items: []Item{
		credentialItem(relabelled),
		credentialItem(elsewhere),
		credentialItem(otherLogin),
		credentialItem(otherEmail),
		cardItem("visa", "5500000000001111"),
		cardItem("VISA", "4000000000002222"),
		identityItem(" me", "me@example.test"),
		identityItem("Me", "other@example.test"),
		noteItem("ROUTER", "\n  admin  \nthird"),
		noteItem("Router", "other"),
		noteItem("Mail", ""),
	}}

	plan := NewPlan(export, entries, nil)

	want := []KindPreview{
		{Kind: vault.KindCredential, Count: 4, Duplicates: 2, Converted: []Conversion{}},
		{Kind: vault.KindCard, Count: 2, Duplicates: 1, Converted: []Conversion{}},
		{Kind: vault.KindIdentity, Count: 2, Duplicates: 1, Converted: []Conversion{}},
		{Kind: vault.KindNote, Count: 3, Duplicates: 1, Converted: []Conversion{}},
	}
	if got := plan.Preview().Kinds; !reflect.DeepEqual(got, want) {
		t.Fatalf("kinds = %+v, want %+v", got, want)
	}
	kept := labelsOf(plan.Items(Options{SkipDuplicates: true}))
	if wantKept := []string{"Mail", "Mail", "VISA", "Me", "Router", "Mail"}; !reflect.DeepEqual(kept, wantKept) {
		t.Fatalf("items without duplicates = %q, want %q", kept, wantKept)
	}
	if all := plan.Items(Options{}); len(all) != len(export.Items) {
		t.Fatalf("items with duplicates = %d, want %d", len(all), len(export.Items))
	}
}

func TestNewPlanCountsOneTimeCodes(t *testing.T) {
	withCode := func(label string) Item {
		return credentialItem(vault.CredentialInput{Label: label, Login: "alex", TOTP: "JBSWY3DPEHPK3PXP"})
	}
	entries := entriesOf(t, withCode("Held"))
	export := Export{Items: []Item{
		withCode("held"),
		withCode("Fresh"),
		credentialItem(vault.CredentialInput{Label: "Plain", Login: "alex"}),
		noteItem("Router", ""),
	}}

	kinds := NewPlan(export, entries, nil).Preview().Kinds

	if credential := kinds[0]; credential.Kind != vault.KindCredential || credential.OneTimeCodes != 2 {
		t.Fatalf("credentials = %+v", credential)
	}
	if note := kinds[1]; note.OneTimeCodes != 0 {
		t.Fatalf("notes = %+v", note)
	}
}

func TestPlanNamesTheNetworkOfACardByItsIssuer(t *testing.T) {
	named := cardItem("Named", "5500000000001111")
	named.Content.Card.Network = vault.NetworkMaestro
	export := Export{Items: []Item{
		cardItem("First", "4111111111111111"),
		named,
		cardItem("Second", "4111111122223333"),
		cardItem("Unknown", "9999999900001111"),
		noteItem("Router", ""),
	}}
	plan := NewPlan(export, nil, nil)

	if issuers, want := plan.Preview().CardIssuers, []string{"41111111", "99999999"}; !reflect.DeepEqual(issuers, want) {
		t.Fatalf("issuers = %q, want %q", issuers, want)
	}
	items := plan.Items(Options{CardNetworks: map[string]vault.CardNetwork{"41111111": vault.NetworkVisa, "55000000": vault.NetworkMastercard}})
	var networks []vault.CardNetwork
	for _, item := range items {
		if item.Card != nil {
			networks = append(networks, item.Card.Network)
		}
	}
	if want := []vault.CardNetwork{vault.NetworkVisa, vault.NetworkMaestro, vault.NetworkVisa, 0}; !reflect.DeepEqual(networks, want) {
		t.Fatalf("networks = %v, want %v", networks, want)
	}
	if again := plan.Items(Options{}); again[0].Card.Network != 0 {
		t.Fatalf("a later plan kept the network: %v", again[0].Card.Network)
	}
}

func TestNewPlanComparesWhatTheListShows(t *testing.T) {
	passport := func(label, expiresOn string) Item {
		identity := vault.IdentityInput{Label: label, Documents: []vault.Document{{Type: vault.DocumentPassport, Number: "P1", ExpiresOn: expiresOn}}}
		return Item{Content: vault.NewItem{Identity: &identity}, Origin: OriginPassport}
	}
	card := func(expiry string) Item {
		item := cardItem("Visa", "4111111111111111")
		item.Content.Card.Expiry = expiry
		return item
	}
	hidden := Item{Content: vault.NewItem{Note: &vault.NoteInput{Label: "Server", Body: "alpha", Hidden: true}}, Origin: OriginNote}
	entries := entriesOf(t, passport("Passport", "2030-01-01"), card("2029-08"), hidden)
	other := hidden
	other.Content.Note = &vault.NoteInput{Label: "server", Body: "beta", Hidden: true}
	export := Export{Items: []Item{
		passport("passport", "2030-01-01"),
		passport("Passport", "2031-05-05"),
		card("2029-08"),
		card("2031-01"),
		other,
	}}

	kinds := NewPlan(export, entries, nil).Preview().Kinds

	want := []KindPreview{
		{Kind: vault.KindCard, Count: 2, Duplicates: 1, Converted: []Conversion{}},
		{Kind: vault.KindIdentity, Count: 2, Duplicates: 1, Converted: []Conversion{{From: OriginPassport, Count: 2}}},
		{Kind: vault.KindNote, Count: 1, Converted: []Conversion{}},
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %+v, want %+v", kinds, want)
	}
}

func TestNewPlanListsSeedsFoundInNotes(t *testing.T) {
	seed := func(label string, words ...string) Item {
		return Item{Content: vault.NewItem{Seed: &vault.SeedInput{Label: label, Format: vault.SeedPhrase, Words: words}}, Origin: OriginNote}
	}
	entries := entriesOf(t, seed("Wallet", "abandon", "about"))
	export := Export{Items: []Item{seed("wallet", "zoo", "wrong"), seed("Wallet", "one", "two", "three")}}

	kinds := NewPlan(export, entries, nil).Preview().Kinds

	want := []KindPreview{{Kind: vault.KindSeed, Count: 2, Duplicates: 1, Converted: []Conversion{{From: OriginNote, Count: 2}}}}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %+v, want %+v", kinds, want)
	}
}

func TestNewPlanFilesFoldersIntoGroups(t *testing.T) {
	tooLong := strings.Repeat("x", vault.MaxGroupNameLength+1)
	export := Export{Items: []Item{
		noteItem("One", "", "work", "Travel"),
		noteItem("Two", "", "travel ", "Personal", "Travel"),
		noteItem("Three", "", tooLong, "  "),
		noteItem("Four", ""),
	}}

	plan := NewPlan(export, nil, heldGroups("Work"))

	want := GroupPreview{New: 2, Existing: 1, Dropped: 2}
	if preview := plan.Preview(); preview.Groups != want || preview.GroupsWithoutDuplicates != want {
		t.Fatalf("groups = %+v and %+v, want %+v", preview.Groups, preview.GroupsWithoutDuplicates, want)
	}
	grouped := groupsOf(plan.Items(Options{Groups: true}))
	if want := [][]string{{"work", "Travel"}, {"Travel", "Personal"}, nil, nil}; !reflect.DeepEqual(grouped, want) {
		t.Fatalf("group names = %q, want %q", grouped, want)
	}
	for _, item := range plan.Items(Options{}) {
		if item.Groups != nil {
			t.Fatalf("groups off, yet %q joins %q", labelOf(item), item.Groups)
		}
	}
}

func TestNewPlanDropsNewGroupsBeyondTheVaultLimit(t *testing.T) {
	held := heldGroups(numberedNames("Held", vault.MaxGroups-2)...)
	export := Export{Items: []Item{
		noteItem("One", "", "New 1"),
		noteItem("Two", "", "New 2", "new 1"),
		noteItem("Three", "", "New 3", "HELD 5"),
	}}

	plan := NewPlan(export, nil, held)

	if got, want := plan.Preview().Groups, (GroupPreview{New: 2, Existing: 1, Dropped: 1}); got != want {
		t.Fatalf("groups = %+v, want %+v", got, want)
	}
	grouped := groupsOf(plan.Items(Options{Groups: true}))
	if want := [][]string{{"New 1"}, {"New 2", "New 1"}, {"HELD 5"}}; !reflect.DeepEqual(grouped, want) {
		t.Fatalf("group names = %q, want %q", grouped, want)
	}
}

func TestNewPlanKeepsTheFirstGroupsOfAnItem(t *testing.T) {
	folders := append([]string{"Folder 0", "FOLDER 0"}, numberedNames("Folder", vault.MaxCredentialGroups+2)...)
	plan := NewPlan(Export{Items: []Item{noteItem("Many", "", folders...)}}, nil, nil)

	items := plan.Items(Options{Groups: true})
	if want := numberedNames("Folder", vault.MaxCredentialGroups); !reflect.DeepEqual(items[0].Groups, want) {
		t.Fatalf("group names = %q, want %q", items[0].Groups, want)
	}
	if got, want := plan.Preview().Groups, (GroupPreview{New: vault.MaxCredentialGroups, Dropped: 2}); got != want {
		t.Fatalf("groups = %+v, want %+v", got, want)
	}
	if _, err := vault.PreviewNewItem(items[0]); err != nil {
		t.Fatalf("the vault refuses the planned item: %v", err)
	}
}

func TestNewPlanFilesGroupsAgainWithoutDuplicates(t *testing.T) {
	entries := entriesOf(t, noteItem("Held note", ""))
	held := heldGroups(numberedNames("Held", vault.MaxGroups-1)...)
	export := Export{Items: []Item{
		noteItem("held NOTE", "", "Only duplicates"),
		noteItem("Fresh", "", "Shared"),
	}}

	plan := NewPlan(export, entries, held)

	preview := plan.Preview()
	if want := (GroupPreview{New: 1, Dropped: 1}); preview.Groups != want {
		t.Fatalf("with duplicates, groups = %+v, want %+v", preview.Groups, want)
	}
	if want := (GroupPreview{New: 1}); preview.GroupsWithoutDuplicates != want {
		t.Fatalf("without duplicates, groups = %+v, want %+v", preview.GroupsWithoutDuplicates, want)
	}
	if grouped, want := groupsOf(plan.Items(Options{Groups: true})), [][]string{{"Only duplicates"}, nil}; !reflect.DeepEqual(grouped, want) {
		t.Fatalf("with duplicates, group names = %q, want %q", grouped, want)
	}
	skipping := plan.Items(Options{Groups: true, SkipDuplicates: true})
	if labels, grouped := labelsOf(skipping), groupsOf(skipping); !reflect.DeepEqual(labels, []string{"Fresh"}) || !reflect.DeepEqual(grouped, [][]string{{"Shared"}}) {
		t.Fatalf("without duplicates, items %q join %q", labels, grouped)
	}
}

func TestNewPlanSkipsWhatTheVaultRefuses(t *testing.T) {
	longLabel := strings.Repeat("l", vault.MaxLabelLength+1)
	longNotes := vault.CredentialInput{Label: "Long notes", Notes: strings.Repeat("n", vault.MaxNotesLength+1)}
	export := Export{
		Format: FormatZIP,
		Items: []Item{
			noteItem("Fine", ""),
			{Content: vault.NewItem{Note: &vault.NoteInput{Label: longLabel}}, Origin: OriginSSHKey},
			credentialItem(longNotes),
			{Origin: OriginIdentity},
		},
		Skipped:     []Skip{{Label: "Mystery", Reason: ReasonUnsupported}},
		Attachments: 3,
		Passkeys:    2,
	}

	preview := NewPlan(export, nil, nil).Preview()

	want := Preview{
		Format:      FormatZIP,
		Items:       5,
		Kinds:       []KindPreview{{Kind: vault.KindNote, Count: 1, Converted: []Conversion{}}},
		CardIssuers: []string{},
		Skipped: []Skip{
			{Label: "Mystery", Reason: ReasonUnsupported},
			{Label: longLabel, Origin: OriginSSHKey, Reason: ReasonTooLong},
			{Label: "Long notes", Origin: OriginLogin, Reason: ReasonTooLong},
			{Origin: OriginIdentity, Reason: ReasonTooLong},
		},
		Attachments: 3,
		Passkeys:    2,
	}
	if !reflect.DeepEqual(preview, want) {
		t.Fatalf("preview = %+v, want %+v", preview, want)
	}
}

func TestNewPlanCountsImportedAndLeftBehindPasskeys(t *testing.T) {
	held := vault.CredentialInput{Label: "Held", Login: "alex"}
	entries := entriesOf(t, credentialItem(held))
	duplicate := held
	duplicate.Passkeys = []vault.Passkey{testPasskey(t, 1), testPasskey(t, 2)}
	fresh := vault.CredentialInput{Label: "Fresh", Passkeys: []vault.Passkey{testPasskey(t, 3)}}
	refused := vault.CredentialInput{Label: "Refused", Notes: strings.Repeat("n", vault.MaxNotesLength+1), Passkeys: []vault.Passkey{testPasskey(t, 4)}}
	export := Export{
		Items:    []Item{credentialItem(duplicate), credentialItem(fresh), credentialItem(refused), noteItem("Router", "")},
		Passkeys: 3,
	}

	preview := NewPlan(export, entries, nil).Preview()

	if preview.ImportedPasskeys != 3 || preview.Passkeys != 4 {
		t.Fatalf("imported %d passkeys and left %d behind", preview.ImportedPasskeys, preview.Passkeys)
	}
}

func TestPreviewListsKindsInOrderWithTheirConversions(t *testing.T) {
	converted := func(item Item, origin Origin) Item {
		item.Origin = origin
		return item
	}
	export := Export{Items: []Item{
		converted(noteItem("Key", ""), OriginSSHKey),
		credentialItem(vault.CredentialInput{Label: "Mail"}),
		converted(identityItem("Passport"), OriginPassport),
		noteItem("Plain", ""),
		converted(noteItem("Bank", ""), OriginBankAccount),
		converted(noteItem("Other key", ""), OriginSSHKey),
		converted(noteItem("Broken card", ""), OriginCard),
		cardItem("Visa", "4111111111111111"),
		identityItem("Me"),
	}}

	kinds := NewPlan(export, nil, nil).Preview().Kinds

	want := []KindPreview{
		{Kind: vault.KindCredential, Count: 1, Converted: []Conversion{}},
		{Kind: vault.KindCard, Count: 1, Converted: []Conversion{}},
		{Kind: vault.KindIdentity, Count: 2, Converted: []Conversion{{From: OriginPassport, Count: 1}}},
		{Kind: vault.KindNote, Count: 5, Converted: []Conversion{{From: OriginSSHKey, Count: 2}, {From: OriginBankAccount, Count: 1}, {From: OriginCard, Count: 1}}},
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %+v, want %+v", kinds, want)
	}
	if empty := NewPlan(Export{}, nil, nil).Preview(); empty.Kinds == nil || len(empty.Kinds) != 0 || empty.Skipped == nil {
		t.Fatalf("an empty plan lists kinds %v and skips %v", empty.Kinds, empty.Skipped)
	}
}

func TestPlanHoldsItsOwnCopies(t *testing.T) {
	identity := vault.IdentityInput{Label: "Me", Emails: []string{"me@example.test"}, Documents: []vault.Document{{Type: vault.DocumentPassport, Number: "P1"}}}
	card := vault.CardInput{Label: "Visa", Number: "4111111111111111", Billing: &vault.Address{City: "Springfield"}}
	passkey := testPasskey(t, 1)
	credential := vault.CredentialInput{Label: "Mail", Websites: []string{"https://mail.example.test"}, Passkeys: []vault.Passkey{passkey}}
	givenPasskey := passkey
	givenPasskey.CredentialID, givenPasskey.UserHandle, givenPasskey.PrivateKey = bytes.Clone(passkey.CredentialID), bytes.Clone(passkey.UserHandle), bytes.Clone(passkey.PrivateKey)
	export := Export{
		Items: []Item{
			{Content: vault.NewItem{Identity: &identity}, Origin: OriginIdentity, Folders: []string{"Travel"}},
			{Content: vault.NewItem{Card: &card}, Origin: OriginCard},
			{Content: vault.NewItem{Credential: &credential}, Origin: OriginLogin},
		},
		Skipped: []Skip{{Label: "Mystery", Reason: ReasonUnsupported}},
	}
	plan := NewPlan(export, nil, nil)
	identity.Label, identity.Emails[0] = "Changed", "changed@example.test"
	card.Billing.City = "Changed"
	credential.Websites[0] = "https://changed.example.test"
	for _, value := range [][]byte{passkey.CredentialID, passkey.UserHandle, passkey.PrivateKey} {
		clear(value)
	}
	export.Items[0].Folders[0] = "Changed"
	export.Skipped[0].Label = "Changed"

	options := Options{Groups: true}
	first := plan.Items(options)
	first[0].Identity.Label = "Mutated"
	first[0].Identity.Emails[0] = "mutated@example.test"
	first[0].Identity.Documents[0].Number = "Mutated"
	first[0].Groups[0] = "Mutated"
	first[1].Card.Billing.City = "Mutated"
	first[2].Credential.Websites[0] = "https://mutated.example.test"
	mutated := first[2].Credential.Passkeys[0]
	for _, value := range [][]byte{mutated.CredentialID, mutated.UserHandle, mutated.PrivateKey} {
		value[0] ^= 0xff
	}
	preview := plan.Preview()
	preview.Skipped[0].Label = "Mutated"
	preview.Kinds[0].Count = 99

	again := plan.Items(options)
	identityAgain, cardAgain, credentialAgain := again[0], again[1].Card, again[2].Credential
	if identityAgain.Identity.Label != "Me" || identityAgain.Identity.Emails[0] != "me@example.test" || identityAgain.Identity.Documents[0].Number != "P1" || identityAgain.Groups[0] != "Travel" || cardAgain.Billing.City != "Springfield" || credentialAgain.Websites[0] != "https://mail.example.test" {
		t.Fatalf("the plan shares memory with what it was given or returned: %+v, %q, %+v, %q", identityAgain.Identity, identityAgain.Groups, cardAgain.Billing, credentialAgain.Websites)
	}
	if !reflect.DeepEqual(credentialAgain.Passkeys, []vault.Passkey{givenPasskey}) {
		t.Fatal("the plan shares passkey memory with what it was given or returned")
	}
	if preview := plan.Preview(); preview.Skipped[0].Label != "Mystery" || preview.Kinds[0].Count != 1 {
		t.Fatalf("the preview shares memory with an earlier one: %+v", preview)
	}
}

func TestNewPlanIsDeterministic(t *testing.T) {
	export := Export{Items: []Item{
		noteItem("One", "", "B", "A"),
		noteItem("Two", "", "C", "a"),
		credentialItem(vault.CredentialInput{Label: "Mail"}),
	}}
	entries := entriesOf(t, noteItem("one", ""))
	first := NewPlan(export, entries, heldGroups("c"))
	second := NewPlan(export, entries, heldGroups("c"))
	options := Options{Groups: true}
	if !reflect.DeepEqual(first.Preview(), second.Preview()) || !reflect.DeepEqual(first.Items(options), second.Items(options)) {
		t.Fatal("two plans of the same export differ")
	}
}
