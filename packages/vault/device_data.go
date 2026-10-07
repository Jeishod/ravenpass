package vault

import "golang.org/x/crypto/chacha20poly1305"

const maxDeviceDataBytes = 32 << 20

// SealDeviceData seals the owner's data that stays on this device, such as generated passwords, so it opens only in
// this vault under the same name. Unlike a device cache it is user data: a holder must not drop it when it fails to
// open, and a key change seals it again with Rekey.SealDeviceData.
func (s *Session) SealDeviceData(name string, plaintext []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	return sealDeviceData(&s.dataKey, s.vaultID, name, plaintext)
}

// OpenDeviceData opens what SealDeviceData sealed for the same name in this vault.
func (s *Session) OpenDeviceData(name string, sealed []byte) ([]byte, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	box, err := decodeEnvelope(sealed, s.vaultID, maxDeviceDataBytes)
	if err != nil {
		return nil, err
	}
	key, err := deriveKey(s.dataKey[:], s.vaultID, deviceDataPurpose(name))
	if err != nil {
		return nil, err
	}
	plaintext, err := openBox(key, box, deviceDataAAD(s.vaultID, name))
	clear(key[:])
	return plaintext, err
}

// SealDeviceData seals device data under the new vault key, which OpenDeviceData opens once the rekey commits.
func (r *Rekey) SealDeviceData(name string, plaintext []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.discarded {
		return nil, ErrLocked
	}
	return sealDeviceData(&r.key.data, r.vaultID, name, plaintext)
}

func sealDeviceData(dataKey *[32]byte, vaultID ID, name string, plaintext []byte) ([]byte, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	if len(plaintext) > maxDeviceDataBytes-chacha20poly1305.Overhead {
		return nil, ErrResourceLimit
	}
	key, err := deriveKey(dataKey[:], vaultID, deviceDataPurpose(name))
	if err != nil {
		return nil, err
	}
	box, err := seal(key, plaintext, deviceDataAAD(vaultID, name))
	clear(key[:])
	if err != nil {
		return nil, err
	}
	return encodeEnvelope(vaultID, box), nil
}

func deviceDataPurpose(name string) string {
	return "device-data/" + name
}

func deviceDataAAD(vaultID ID, name string) []byte {
	hash := headerHash(vaultID)
	result := make([]byte, 0, 33+len(name))
	result = append(result, 6)
	result = append(result, hash[:]...)
	return append(result, name...)
}
