package api

import (
	"reflect"
	"testing"
)

func TestHostOnlyCallsStayOffTheBoundService(t *testing.T) {
	bound := reflect.TypeOf(&Service{})
	for _, name := range []string{"FollowVaultFile", "LockAway", "ReceiveCodeSetup"} {
		if _, found := bound.MethodByName(name); found {
			t.Errorf("the web page can call %s", name)
		}
		if _, found := reflect.TypeOf(HostControls{}).MethodByName(name); !found {
			t.Errorf("the host cannot call %s", name)
		}
	}
}
