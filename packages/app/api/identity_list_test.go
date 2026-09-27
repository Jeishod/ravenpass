package api

import (
	"sync/atomic"
	"testing"
)

// countedList counts how often the service tells it the choice changed.
type countedList struct {
	changes atomic.Int32
}

func (l *countedList) Changed() { l.changes.Add(1) }

func TestAHostWithAnIdentityListOffersIt(t *testing.T) {
	if newServiceOnHost(t, Host{}).Capabilities().IdentityList {
		t.Fatal("a host without an identity list offers one")
	}
	if !newServiceOnHost(t, Host{IdentityList: &countedList{}}).Capabilities().IdentityList {
		t.Fatal("a host with an identity list does not offer it")
	}
}

func TestTheIdentityListIsOffUntilChosenAndEachChoiceReachesTheHost(t *testing.T) {
	list := &countedList{}
	service := newServiceOnHost(t, Host{IdentityList: list})
	current, err := service.GetIdentityList()
	if err != nil || current.Enabled {
		t.Fatalf("default identity list = %+v, error = %v", current, err)
	}
	if err := service.SetIdentityList(true); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetIdentityList(); !current.Enabled {
		t.Fatalf("identity list after turning it on = %+v", current)
	}
	if err := service.SetIdentityList(false); err != nil {
		t.Fatal(err)
	}
	if current, _ = service.GetIdentityList(); current.Enabled {
		t.Fatalf("identity list after turning it off = %+v", current)
	}
	if got := list.changes.Load(); got != 2 {
		t.Fatalf("the host heard of %d changes, want 2", got)
	}
}
