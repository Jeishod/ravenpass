package vaultservice

import "testing"

func TestUnlockedFollowsTheOpenVault(t *testing.T) {
	service := newTestService(t, &memoryFiles{}, newMemoryKeys())
	if service.Unlocked() {
		t.Fatal("a service without a vault reports one open")
	}
	createTestVault(t, service)
	if !service.Unlocked() {
		t.Fatal("a created vault is not open")
	}
	service.Lock()
	if service.Unlocked() {
		t.Fatal("a locked vault reports itself open")
	}
}
