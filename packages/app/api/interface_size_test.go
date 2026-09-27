package api

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/dortanes/ravenpass/packages/app/localfile"
)

func TestInterfaceSizeIsRecorded(t *testing.T) {
	home := t.TempDir()
	service, _ := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, localfile.DefaultVaultName))
	size, err := service.GetInterfaceSize()
	if err != nil || size.Percent != 100 || !slices.Contains(size.Offered, 130) {
		t.Fatalf("interface size = %+v, %v", size, err)
	}
	if err := service.SetInterfaceSize(130); err != nil {
		t.Fatal(err)
	}
	if err := service.SetInterfaceSize(131); err == nil {
		t.Fatal("an unoffered size was recorded")
	}
	if size, err := service.GetInterfaceSize(); err != nil || size.Percent != 130 {
		t.Fatalf("interface size after a change = %+v, %v", size, err)
	}
}

func TestWebsitesOpenThroughTheHost(t *testing.T) {
	home := t.TempDir()
	service, _ := newServiceWithStorage(t, filepath.Join(home, "storage.json"), filepath.Join(home, localfile.DefaultVaultName))
	var opened []string
	service.openURL = func(address string) error {
		opened = append(opened, address)
		return nil
	}
	if err := service.OpenWebsite("bank.example"); err != nil {
		t.Fatal(err)
	}
	if err := service.OpenWebsite("javascript:alert(1)"); err == nil {
		t.Fatal("an address that is not a web page was opened")
	}
	if !slices.Equal(opened, []string{"https://bank.example"}) {
		t.Fatalf("opened = %v", opened)
	}
}
