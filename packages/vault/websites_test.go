package vault

import (
	"errors"
	"reflect"
	"testing"
)

// credentialRecordOf is a credential record with the given websites, password, passkeys and apps fields and empty text elsewhere.
func credentialRecordOf(websites, password, passkeys, apps []byte) []byte {
	empty := encodeBytes(nil)
	return encodeArray(encodeUint(recordSchemaCredential), websites, empty, empty, password, empty, empty, passkeys, apps)
}

func TestEveryCredentialIsWrittenInOneLayout(t *testing.T) {
	tests := []struct {
		name  string
		input CredentialInput
	}{
		{"no website", CredentialInput{Login: "alex", Password: "secret", Notes: "line"}},
		{"one website", CredentialInput{Websites: []string{"https://example.com"}, Login: "alex", Password: "secret"}},
		{"one website and a code", CredentialInput{Websites: []string{"https://example.com"}, Password: "secret", TOTP: standardSecret}},
		{"two websites", CredentialInput{Websites: []string{"https://example.com", "https://admin.example.com"}, Email: "alex@example.com", Password: "secret"}},
		{"every website and a code", CredentialInput{Websites: numberedWebsites(MaxCredentialWebsites), Password: "secret", TOTP: standardSecret}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plaintext, err := encodeCredentialRecord(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if plaintext[0] != 0x89 || plaintext[1] != recordSchemaCredential {
				t.Fatalf("written as % x…, want a nine-field schema %d record", plaintext[:2], recordSchemaCredential)
			}
			decoded, err := decodeCredentialRecord(plaintext)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, test.input) {
				t.Fatalf("read back as %+v", decoded)
			}
		})
	}
}

func TestCredentialRecordOfAnUnknownSchemaIsUnsupported(t *testing.T) {
	empty := encodeBytes(nil)
	website := encodeBytes([]byte("https://example.com"))
	websites := encodeArray(encodeTexts(numberedWebsites(2))...)
	tests := map[string][]byte{
		"schema 1":  encodeArray(encodeUint(1), website, empty, empty, empty, empty),
		"schema 2":  encodeArray(encodeUint(2), website, empty, empty, empty, empty, empty),
		"schema 12": encodeArray(encodeUint(12), websites, empty, empty, empty, empty, empty),
		"schema 13": encodeArray(encodeUint(13), websites, empty, empty, empty, empty, empty, encodeArray()),
	}
	for name, record := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCredentialRecord(record); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("got %v, want ErrUnsupported", err)
			}
		})
	}
}

func TestWebsiteFieldOutsideItsShapeIsMalformed(t *testing.T) {
	none, empty := encodeArray(), encodeBytes(nil)
	tests := []struct {
		name   string
		record []byte
	}{
		{"over the website limit", credentialRecordOf(encodeArray(encodeTexts(numberedWebsites(MaxCredentialWebsites+1))...), empty, none, none)},
		{"one text for its websites", credentialRecordOf(encodeBytes([]byte("https://example.com")), empty, none, none)},
		{"a website that is not text", credentialRecordOf(encodeArray(encodeBytes([]byte("\xff"))), empty, none, none)},
		{"without its apps", encodeArray(encodeUint(recordSchemaCredential), none, empty, empty, empty, empty, empty, none)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeCredentialRecord(test.record); !errors.Is(err, ErrMalformed) {
				t.Fatalf("got %v, want ErrMalformed", err)
			}
		})
	}
}

func TestSeveralWebsitesKeepTheirOrderAcrossAReopen(t *testing.T) {
	websites := []string{"https://app.example.com", "https://admin.example.com", "androidapp://com.example"}
	created, ids := vaultWith(t,
		CredentialInput{Label: "Example", Websites: websites, Password: "one"},
	)
	container, _, err := created.Session.CurrentContainer()
	if err != nil {
		t.Fatal(err)
	}
	created.Session.Lock()
	session, err := OpenWithRecovery(container, created.RecoveryPhrase)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Lock()
	if got := selectCredential(t, session, ids[0]).Websites; !reflect.DeepEqual(got, websites) {
		t.Fatalf("websites read back as %q", got)
	}
	if entry := listedEntry(t, session, ids[0]); entry.Site != "app.example.com" {
		t.Fatalf("site = %q", entry.Site)
	}
}
