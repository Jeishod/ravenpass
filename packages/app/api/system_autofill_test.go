package api

import (
	"errors"
	"testing"
)

// shownSystemAutofill reports a fixed status and counts the screens it shows, each failing with err.
type shownSystemAutofill struct {
	status SystemAutofillStatus
	shown  int
	err    error
}

func (s *shownSystemAutofill) Status() SystemAutofillStatus { return s.status }

func (s *shownSystemAutofill) Choose() error {
	s.shown++
	return s.err
}

func TestAHostWhoseSystemChoosesItsAutofillOffersTheChoice(t *testing.T) {
	if newServiceOnHost(t, Host{}).Capabilities().SystemAutofill {
		t.Fatal("a host without the port offers system autofill")
	}
	if !newServiceOnHost(t, Host{SystemAutofill: &shownSystemAutofill{}}).Capabilities().SystemAutofill {
		t.Fatal("a host with the port does not offer system autofill")
	}
}

func TestTheSystemAutofillStatusIsTheDevices(t *testing.T) {
	want := SystemAutofillStatus{Autofill: true, PasskeyProviders: true}
	status, err := newServiceOnHost(t, Host{SystemAutofill: &shownSystemAutofill{status: want}}).GetSystemAutofill()
	if err != nil || status != want {
		t.Fatalf("status = %+v, %v; want %+v", status, err, want)
	}
	if status, err := newServiceOnHost(t, Host{}).GetSystemAutofill(); err != nil || status != (SystemAutofillStatus{}) {
		t.Fatalf("a host without the port reports %+v, %v", status, err)
	}
}

func TestTheHostIsHeldWhileTheSystemAutofillScreenShows(t *testing.T) {
	system := &shownSystemAutofill{}
	held, released := 0, 0
	service := newServiceOnHost(t, Host{
		SystemAutofill: system,
		Hold: func() (release func()) {
			held++
			return func() { released++ }
		},
	})
	if err := service.ChooseSystemAutofill(); err != nil {
		t.Fatal(err)
	}
	if system.shown != 1 || held != 1 || released != 1 {
		t.Fatalf("shown %d, held %d, released %d; want 1 each", system.shown, held, released)
	}
}

func TestASystemAutofillScreenThatDoesNotShowFailsInWords(t *testing.T) {
	service := newServiceOnHost(t, Host{SystemAutofill: &shownSystemAutofill{err: errors.New("no screen")}})
	if err := service.ChooseSystemAutofill(); err == nil || err.Error() != fail(failureGeneral).Error() {
		t.Fatalf("choosing = %v, want %s", err, failureGeneral)
	}
	if err := newServiceOnHost(t, Host{}).ChooseSystemAutofill(); err == nil || err.Error() != fail(failureGeneral).Error() {
		t.Fatalf("choosing on a host without the port = %v, want %s", err, failureGeneral)
	}
}
