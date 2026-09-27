package linkproto

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestCaptureRequestsCarryTheirFields(t *testing.T) {
	tests := []struct {
		payload string
		want    Request
	}{
		{
			`{"id":1,"type":"capture","origin":"https://example.com","account":"alex","password":"new horse","current":"old horse"}`,
			Request{ID: 1, Type: RequestCapture, Origin: "https://example.com", Account: "alex", Password: "new horse", Current: "old horse"},
		},
		{
			`{"id":2,"type":"capture","origin":"https://example.com","account":"","password":"horse"}`,
			Request{ID: 2, Type: RequestCapture, Origin: "https://example.com", Password: "horse"},
		},
		{`{"id":3,"type":"review","pending":"p1"}`, Request{ID: 3, Type: RequestReview, Pending: "p1"}},
		{
			`{"id":4,"type":"save","pending":"p1","target":"","account":"alex@example.com","name":"Example"}`,
			Request{ID: 4, Type: RequestSave, Pending: "p1", Account: "alex@example.com", Name: "Example"},
		},
		{
			`{"id":5,"type":"save","pending":"p1","target":"0102","account":"alex","name":"example.com"}`,
			Request{ID: 5, Type: RequestSave, Pending: "p1", Target: "0102", Account: "alex", Name: "example.com"},
		},
		{`{"id":6,"type":"discard","pending":"p1"}`, Request{ID: 6, Type: RequestDiscard, Pending: "p1"}},
	}
	for _, test := range tests {
		request, err := ParseRequest([]byte(test.payload))
		if err != nil || !reflect.DeepEqual(request, test.want) {
			t.Fatalf("%s parsed as %+v, error = %v", test.payload, request, err)
		}
	}
	for _, payload := range []string{
		`{"id":1,"type":"capture","origin":"https://example.com","password":["horse"]}`,
		`{"id":1,"type":"capture","origin":"https://example.com","password":"horse","current":1}`,
		`{"id":1,"type":"review","pending":{}}`,
		`{"id":1,"type":"save","pending":"p1","target":2}`,
		`{"id":1,"type":"save","pending":"p1","name":false}`,
	} {
		if _, err := ParseRequest([]byte(payload)); !errors.Is(err, ErrMalformed) {
			t.Errorf("payload %q: got %v, want ErrMalformed", payload, err)
		}
	}
}

func TestCaptureResultsEncodeAsTheExtensionReadsThem(t *testing.T) {
	tests := []struct {
		result any
		want   string
	}{
		{
			CaptureOffer{
				State: CaptureReady, Pending: "p1", Site: "github.com", Account: "alex", Name: "github.com",
				Targets: []SaveTarget{
					{Credential: "a1", Label: "GitHub", Account: "alex", Action: SaveUpdate},
					{Credential: "b2", Label: "Mail", Account: "alex@example.com", Action: SaveAddSite},
				},
				Suggested: "a1",
			},
			`{"state":"ready","pending":"p1","site":"github.com","account":"alex","name":"github.com",` +
				`"targets":[{"credential":"a1","label":"GitHub","account":"alex","action":"update"},` +
				`{"credential":"b2","label":"Mail","account":"alex@example.com","action":"add-site"}],"suggested":"a1"}`,
		},
		{
			CaptureOffer{State: CaptureNone, Site: "github.com", Account: "alex", Name: "github.com", Targets: []SaveTarget{}},
			`{"state":"none","site":"github.com","account":"alex","name":"github.com","targets":[],"suggested":""}`,
		},
		{
			CaptureOffer{State: CaptureLocked, Pending: "p2", Site: "github.com", Name: "github.com", Targets: []SaveTarget{}},
			`{"state":"locked","pending":"p2","site":"github.com","account":"","name":"github.com","targets":[],"suggested":""}`,
		},
		{Saved{Saved: SavedCreated}, `{"saved":"created"}`},
		{Saved{Saved: SavedUpdated}, `{"saved":"updated"}`},
	}
	for _, test := range tests {
		encoded, err := json.Marshal(test.result)
		if err != nil || string(encoded) != test.want {
			t.Fatalf("%+v encodes as %s, error = %v", test.result, encoded, err)
		}
	}
}
