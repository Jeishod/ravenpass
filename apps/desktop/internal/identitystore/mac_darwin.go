//go:build darwin && cgo

package identitystore

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AuthenticationServices -framework Foundation
#include "mac_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"github.com/dortanes/ravenpass/packages/app/autofill"
)

var errWrite = errors.New("the credential identity store did not take the change")

// listedIdentity is an identity as the Objective-C side reads it. Data travels as Base64.
type listedIdentity struct {
	Kind         autofill.IdentityKind `json:"kind"`
	Site         string                `json:"site"`
	User         string                `json:"user"`
	Record       string                `json:"record"`
	CredentialID []byte                `json:"credentialID,omitempty"`
	UserHandle   []byte                `json:"userHandle,omitempty"`
}

// Enabled reports whether the owner turned Ravenpass's AutoFill extension on.
func (Mac) Enabled() bool { return C.ravenpass_identities_enabled() != 0 }

func (Mac) Replace(identities []autofill.CredentialIdentity) error {
	listed := make([]listedIdentity, len(identities))
	for i, identity := range identities {
		listed[i] = listedIdentity{
			Kind: identity.Kind, Site: identity.Site, User: identity.User, Record: identity.Record,
			CredentialID: identity.CredentialID, UserHandle: identity.UserHandle,
		}
	}
	text, err := json.Marshal(listed)
	if err != nil {
		return err
	}
	if C.ravenpass_identities_replace((*C.char)(unsafe.Pointer(&text[0])), C.size_t(len(text))) == 0 {
		return errWrite
	}
	return nil
}

func (Mac) RemoveAll() error {
	if C.ravenpass_identities_remove_all() == 0 {
		return errWrite
	}
	return nil
}
