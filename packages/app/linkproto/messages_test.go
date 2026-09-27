package linkproto

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAShareRequestNamesAnIdentityAndOneOfItsFiles(t *testing.T) {
	request, err := ParseRequest([]byte(`{"id":4,"type":"share","identity":"0102","file":"photo","origin":"https://example.com"}`))
	want := Request{ID: 4, Type: RequestShare, Identity: "0102", File: "photo", Origin: "https://example.com"}
	if err != nil || !reflect.DeepEqual(request, want) {
		t.Fatalf("share request = %+v, error = %v", request, err)
	}
	for _, payload := range []string{
		`{"id":1,"type":"share","identity":16,"file":"photo","origin":"https://example.com"}`,
		`{"id":1,"type":"share","identity":"0102","file":["photo"],"origin":"https://example.com"}`,
	} {
		if _, err := ParseRequest([]byte(payload)); !errors.Is(err, ErrMalformed) {
			t.Errorf("payload %q: got %v, want ErrMalformed", payload, err)
		}
	}
}

func TestTheLinkedMessageAndTheSessionGreetingNameTheLanguageAndTheSignInStyle(t *testing.T) {
	linked, err := LinkedMessage(Greeting{Language: "ru", SignIn: "field"})
	if err != nil || string(linked) != `{"type":"linked","language":"ru","signIn":"field"}` {
		t.Fatalf("linked message = %s, error = %v", linked, err)
	}
	greeting, err := SessionGreeting(Greeting{Language: "ru", SignIn: "card"})
	if err != nil || string(greeting) != `{"language":"ru","signIn":"card"}` {
		t.Fatalf("session greeting = %s, error = %v", greeting, err)
	}
}

func joined(parts [][]byte) []byte {
	return bytes.Join(parts, nil)
}

func assertPartSizes(t *testing.T, parts [][]byte) {
	t.Helper()
	for i, part := range parts {
		if len(part) == 0 || len(part) > MaxPlaintextBytes {
			t.Fatalf("part %d holds %d bytes", i, len(part))
		}
	}
}

func TestAResponseThatFitsOneMessageIsSentWhole(t *testing.T) {
	for _, response := range []Response{
		{ID: 1, Result: Status{Vault: VaultLocked}},
		{ID: 2, Error: ErrorDeclined},
		{ID: 3, Progress: ProgressConfirmOnDevice},
		{ID: 4, Result: struct{}{}},
	} {
		frames, err := Frames(response)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if len(frames) != 1 || !bytes.Equal(frames[0], encoded) {
			t.Fatalf("%+v is sent as %q", response, frames)
		}
	}
	frames, err := Frames(Response{ID: 5, Progress: ProgressConfirmInRavenpass})
	if err != nil || string(frames[0]) != `{"id":5,"progress":"confirm-in-ravenpass"}` {
		t.Fatalf("a progress message = %q, error = %v", frames, err)
	}
}

func TestALargerResultFollowsAHeadInParts(t *testing.T) {
	thumbnail := strings.Repeat("A", 40_000)
	result := Identities{Identities: []Identity{
		{ID: "one", Label: "Alex", Thumbnail: thumbnail, Files: []IdentityFile{}},
		{ID: "two", Label: "Sam", Thumbnail: thumbnail, Files: []IdentityFile{{ID: "photo", Kind: FilePhoto, Name: "photo.jpg", MediaType: "image/jpeg", Thumbnail: thumbnail}}},
	}}
	frames, err := Frames(Response{ID: 9, Result: result})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 || string(frames[0]) != `{"id":9,"parts":2}` {
		t.Fatalf("head = %s after which %d frames follow", frames[0], len(frames)-1)
	}
	assertPartSizes(t, frames[1:])
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(joined(frames[1:]), body) {
		t.Fatal("the parts do not join into the result's encoding")
	}
}

func TestAFileAlwaysFollowsAHeadThatDescribesIt(t *testing.T) {
	tests := []struct {
		size  int
		parts int
	}{
		{1, 1},
		{MaxPlaintextBytes, 1},
		{MaxPlaintextBytes + 1, 2},
		{960 << 10, 16},
	}
	for _, test := range tests {
		content := bytes.Repeat([]byte{0x25}, test.size)
		file := SharedFile{Name: "passport.pdf", MediaType: "application/pdf", Size: test.size}
		frames, err := FileFrames(3, file, content)
		if err != nil {
			t.Fatal(err)
		}
		var head struct {
			ID     int64      `json:"id"`
			Result SharedFile `json:"result"`
			Parts  int        `json:"parts"`
		}
		if err := json.Unmarshal(frames[0], &head); err != nil {
			t.Fatal(err)
		}
		if head.ID != 3 || head.Result != file || head.Parts != test.parts || len(frames) != test.parts+1 {
			t.Fatalf("a file of %d bytes: head %s and %d parts", test.size, frames[0], len(frames)-1)
		}
		assertPartSizes(t, frames[1:])
		if !bytes.Equal(joined(frames[1:]), content) {
			t.Fatalf("a file of %d bytes does not join back", test.size)
		}
	}
	frames, err := FileFrames(8, SharedFile{Name: "photo.jpg", MediaType: "image/jpeg", Size: 2}, []byte{0xff, 0xd8})
	if err != nil || string(frames[0]) != `{"id":8,"result":{"name":"photo.jpg","mediaType":"image/jpeg","size":2},"parts":1}` {
		t.Fatalf("head = %s, error = %v", frames[0], err)
	}
}

func TestAnIdentityFileNamesItsDocumentOnlyForAScan(t *testing.T) {
	encoded, err := json.Marshal([]IdentityFile{
		{ID: "photo", Kind: FilePhoto, Name: "photo.jpg", MediaType: "image/jpeg", Thumbnail: "/9j/"},
		{ID: "0a0b", Kind: FileScan, Name: "passport.pdf", MediaType: "application/pdf", Document: &Document{Type: "passport"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"photo","kind":"photo","name":"photo.jpg","mediaType":"image/jpeg","thumbnail":"/9j/"},` +
		`{"id":"0a0b","kind":"scan","name":"passport.pdf","mediaType":"application/pdf","document":{"type":"passport","label":""},"thumbnail":""}]`
	if string(encoded) != want {
		t.Fatalf("files encode as %s", encoded)
	}
}
