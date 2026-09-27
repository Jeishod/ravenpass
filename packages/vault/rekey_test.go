package vault

import (
	"bytes"
	"errors"
	"testing"
)

// rekeyed commits a rekey of session and returns the new phrase and the file before and after.
func rekeyed(t *testing.T, session *Session) (phrase string, before, after []byte, rekey *Rekey) {
	t.Helper()
	before, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	rekey, err = session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	after, _, err = session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	return rekey.RecoveryPhrase(), before, after, rekey
}

func TestRekeySealsTheVaultForANewPhraseAndKeepsOldCopiesOnTheOldOne(t *testing.T) {
	created, ids := vaultWith(t,
		CredentialInput{Label: "GitHub", Login: "alex", Password: "example-1-DO-NOT-USE"},
		CredentialInput{Label: "Mail", Email: "alex@example.test"},
	)
	session := created.Session
	defer session.Lock()
	pending, noteID, err := session.PrepareCreateNote(NoteInput{Label: "Door", Body: "code 0000"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	listed, err := session.List()
	if err != nil {
		t.Fatal(err)
	}
	oldHead, err := session.Head()
	if err != nil {
		t.Fatal(err)
	}

	phrase, before, after, _ := rekeyed(t, session)
	if phrase == "" || phrase == created.RecoveryPhrase {
		t.Fatal("the rekey kept the recovery phrase")
	}
	head, err := session.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head.Revision != oldHead.Revision+1 || head.PreviousHash != oldHead.Hash {
		t.Fatalf("the rekeyed version %+v does not follow %+v", head, oldHead)
	}

	opened, err := OpenWithRecovery(after, phrase)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Lock()
	reopened, err := opened.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened) != len(listed) {
		t.Fatalf("the rekeyed vault lists %d items, want %d", len(reopened), len(listed))
	}
	if got := selectCredential(t, opened, ids[0]); got.Password != "example-1-DO-NOT-USE" {
		t.Fatalf("the rekeyed credential reads %q", got.Password)
	}
	oldRaw, err := parseContainer(before)
	if err != nil {
		t.Fatal(err)
	}
	newRaw, err := parseContainer(after)
	if err != nil {
		t.Fatal(err)
	}
	for i := range newRaw.records {
		if bytes.Equal(newRaw.records[i].ciphertext, oldRaw.records[i].ciphertext) {
			t.Fatalf("record %d kept its ciphertext", i)
		}
	}
	for _, entry := range opened.entries {
		if entry.id == noteID && entry.revision != 1 {
			t.Fatalf("the note moved to revision %d", entry.revision)
		}
	}

	if _, err := OpenWithRecovery(after, created.RecoveryPhrase); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("the old phrase on the rekeyed vault: got %v, want ErrAuthentication", err)
	}
	old, err := OpenWithRecovery(before, created.RecoveryPhrase)
	if err != nil {
		t.Fatalf("an older copy no longer opens with its phrase: %v", err)
	}
	old.Lock()
	if _, err := OpenWithRecovery(before, phrase); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("the new phrase on an older copy: got %v, want ErrAuthentication", err)
	}

	next, _, err := session.PrepareCreate(CredentialInput{Label: "After"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(next); err != nil {
		t.Fatal(err)
	}
	later, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenWithRecovery(later, phrase); err != nil {
		t.Fatalf("a save after the rekey does not open with the new phrase: %v", err)
	} else {
		opened.Lock()
	}
}

func TestRekeyReplacesTheKeyDeviceEnvelopesHold(t *testing.T) {
	deviceKey := bytes.Repeat([]byte{0x21}, 32)
	created, _ := vaultWith(t, CredentialInput{Label: "GitHub", Password: "example-2-DO-NOT-USE"})
	session := created.Session
	defer session.Lock()
	oldEnvelope := wrapFor(t, session, deviceKey)
	oldHead, err := session.Head()
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	rekey, err := session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	newEnvelope, err := rekey.WrapDeviceKey(deviceKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.VerifyDeviceKey(deviceKey, oldEnvelope); err != nil {
		t.Fatalf("the staged rekey changed the open vault's key: %v", err)
	}
	pending, err := session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(pending); err != nil {
		t.Fatal(err)
	}
	after, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}

	if err := CheckDeviceEnvelope(after, oldEnvelope); !errors.Is(err, ErrKeyReplaced) {
		t.Fatalf("checking the old envelope: got %v, want ErrKeyReplaced", err)
	}
	if _, _, err := OpenWithDevice(after, deviceKey, oldEnvelope, ptrWitness(WitnessFor(oldHead)), func(Witness) error { return nil }); !errors.Is(err, ErrKeyReplaced) {
		t.Fatalf("opening with the old envelope: got %v, want ErrKeyReplaced", err)
	}
	if err := session.VerifyDeviceKey(deviceKey, oldEnvelope); !errors.Is(err, ErrKeyReplaced) {
		t.Fatalf("verifying the old envelope: got %v, want ErrKeyReplaced", err)
	}
	if err := CheckDeviceEnvelope(after, newEnvelope); err != nil {
		t.Fatal(err)
	}
	if err := session.VerifyDeviceKey(deviceKey, newEnvelope); err != nil {
		t.Fatal(err)
	}
	opened, _, err := OpenWithDevice(after, deviceKey, newEnvelope, ptrWitness(WitnessFor(oldHead)), func(Witness) error { return nil })
	if err != nil {
		t.Fatalf("the rewrapped envelope does not open the rekeyed vault: %v", err)
	}
	opened.Lock()
	if _, _, err := OpenWithDevice(before, deviceKey, newEnvelope, ptrWitness(WitnessFor(oldHead)), nil); !errors.Is(err, ErrKeyReplaced) {
		t.Fatalf("the new envelope on an older copy: got %v, want ErrKeyReplaced", err)
	}
	wrongKey := bytes.Repeat([]byte{0x22}, 32)
	if _, _, err := OpenWithDevice(after, wrongKey, newEnvelope, ptrWitness(WitnessFor(oldHead)), nil); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("a wrong device key on a current envelope: got %v, want ErrAuthentication", err)
	}
}

func TestFollowingARekeyedFileNeedsTheNewPhrase(t *testing.T) {
	created, _ := vaultWith(t, CredentialInput{Label: "GitHub"})
	session := created.Session
	defer session.Lock()
	before, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	other, err := OpenWithRecovery(before, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Lock()
	_, _, after, _ := rekeyed(t, session)
	if _, err := other.Follow(after, func(Witness) error { return nil }); !errors.Is(err, ErrKeyReplaced) {
		t.Fatalf("following the rekeyed file: got %v, want ErrKeyReplaced", err)
	}
}

func TestAnAbortedRekeyKeepsTheVaultKey(t *testing.T) {
	deviceKey := bytes.Repeat([]byte{0x31}, 32)
	created, _ := vaultWith(t, CredentialInput{Label: "GitHub"})
	session := created.Session
	defer session.Lock()
	envelope := wrapFor(t, session, deviceKey)
	rekey, err := session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Abort(pending); err != nil {
		t.Fatal(err)
	}
	if pending.rekeyed != nil {
		t.Fatal("the aborted save kept the new keys")
	}
	if err := session.VerifyDeviceKey(deviceKey, envelope); err != nil {
		t.Fatalf("the aborted rekey changed the key: %v", err)
	}
	next, _, err := session.PrepareCreate(CredentialInput{Label: "Mail"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(next); err != nil {
		t.Fatal(err)
	}
	container, _, err := session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenWithRecovery(container, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	opened.Lock()
}

func TestARekeyOnlyItsOwnPhraseVerifiesAndADiscardedOneDoesNothing(t *testing.T) {
	created, _ := vaultWith(t)
	session := created.Session
	defer session.Lock()
	rekey, err := session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	if err := rekey.VerifyPhrase(rekey.RecoveryPhrase()); err != nil {
		t.Fatal(err)
	}
	if err := rekey.VerifyPhrase(created.RecoveryPhrase); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("the old phrase: got %v, want ErrAuthentication", err)
	}
	if err := rekey.VerifyPhrase("not a phrase"); !errors.Is(err, ErrInvalidPhrase) {
		t.Fatalf("a malformed phrase: got %v, want ErrInvalidPhrase", err)
	}
	rekey.Discard()
	if rekey.RecoveryPhrase() != "" || rekey.key.data != [32]byte{} {
		t.Fatal("the discarded rekey kept its phrase or key")
	}
	if _, err := rekey.WrapDeviceKey(bytes.Repeat([]byte{1}, 32)); !errors.Is(err, ErrLocked) {
		t.Fatalf("wrapping with a discarded rekey: got %v, want ErrLocked", err)
	}
	if _, err := session.PrepareRekey(rekey); !errors.Is(err, ErrLocked) {
		t.Fatalf("preparing a discarded rekey: got %v, want ErrLocked", err)
	}
	other, _ := vaultWith(t)
	defer other.Session.Lock()
	foreign, err := other.Session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.PrepareRekey(foreign); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("another vault's rekey: got %v, want ErrInvalidInput", err)
	}
}

func TestLockingDuringARekeyClearsTheNewKeys(t *testing.T) {
	created, _ := vaultWith(t, CredentialInput{Label: "GitHub"})
	session := created.Session
	rekey, err := session.BeginRekey()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := session.PrepareRekey(rekey)
	if err != nil {
		t.Fatal(err)
	}
	session.Lock()
	if pending.rekeyed != nil || pending.container != nil {
		t.Fatal("the locked session left the prepared rekey holding its keys")
	}
	if _, err := session.BeginRekey(); !errors.Is(err, ErrLocked) {
		t.Fatalf("a rekey of a locked vault: got %v, want ErrLocked", err)
	}
}
