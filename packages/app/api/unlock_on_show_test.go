package api

import "testing"

func TestAHostThatReportsItsViewOffersUnlockOnShow(t *testing.T) {
	if !newServiceOnHost(t, Host{UnlockOnShow: &UnlockOnShow{}}).Capabilities().UnlockOnShow {
		t.Fatal("a host that reports its view does not offer the unlock prompt")
	}
	if newServiceOnHost(t, Host{}).Capabilities().UnlockOnShow {
		t.Fatal("a host that does not report its view offers the unlock prompt")
	}
}

func TestTheLockedScreenAsksOnAColdStart(t *testing.T) {
	shows := &UnlockOnShow{}
	service := newServiceOnHost(t, Host{UnlockOnShow: shows})
	shows.Shown()
	if !service.PromptsUnlock() {
		t.Fatal("the locked screen of an app that just came into view does not ask")
	}
}

func TestTheOwnersLockHoldsThePromptBackUntilTheAppLeavesView(t *testing.T) {
	shows := &UnlockOnShow{}
	service := newServiceOnHost(t, Host{UnlockOnShow: shows})
	shows.Shown()
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	if service.PromptsUnlock() {
		t.Fatal("the locked screen asks right after the owner locked the vault")
	}
	shows.Shown()
	if service.PromptsUnlock() {
		t.Fatal("a repeated report of the app in view let the locked screen ask")
	}
	shows.Hidden()
	shows.Shown()
	if !service.PromptsUnlock() {
		t.Fatal("the locked screen does not ask once the app returns from the background")
	}
}

func TestALockWhileTheAppIsHiddenLeavesThePromptForItsReturn(t *testing.T) {
	shows := &UnlockOnShow{}
	service := newServiceOnHost(t, Host{UnlockOnShow: shows})
	shows.Shown()
	shows.Hidden()
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	shows.Shown()
	if !service.PromptsUnlock() {
		t.Fatal("an automatic lock in the background kept the locked screen from asking on return")
	}
}

func TestALockThatRacesTheAppLeavingViewLeavesThePromptForItsReturn(t *testing.T) {
	shows := &UnlockOnShow{}
	service := newServiceOnHost(t, Host{UnlockOnShow: shows})
	shows.Shown()
	if err := service.Lock(); err != nil {
		t.Fatal(err)
	}
	shows.Hidden()
	shows.Shown()
	if !service.PromptsUnlock() {
		t.Fatal("a screen lock reported before the app left view kept the locked screen from asking")
	}
}
