package vault

import "golang.org/x/crypto/chacha20poly1305"

const maxDeviceCacheBytes = 32 << 20

// SealDeviceCache seals device-side data that opens only in this vault under the same name, a fixed purpose and never user data.
func (s *Session) SealDeviceCache(name string, plaintext []byte) ([]byte, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	if len(plaintext) > maxDeviceCacheBytes-chacha20poly1305.Overhead {
		return nil, ErrResourceLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	key, err := deriveKey(s.dataKey[:], s.vaultID, deviceCachePurpose(name))
	if err != nil {
		return nil, err
	}
	box, err := seal(key, plaintext, deviceCacheAAD(s.vaultID, name))
	clear(key[:])
	if err != nil {
		return nil, err
	}
	return encodeEnvelope(s.vaultID, box), nil
}

// OpenDeviceCache opens what SealDeviceCache sealed for the same name in this vault.
func (s *Session) OpenDeviceCache(name string, sealed []byte) ([]byte, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	box, err := decodeEnvelope(sealed, s.vaultID, maxDeviceCacheBytes)
	if err != nil {
		return nil, err
	}
	key, err := deriveKey(s.dataKey[:], s.vaultID, deviceCachePurpose(name))
	if err != nil {
		return nil, err
	}
	plaintext, err := openBox(key, box, deviceCacheAAD(s.vaultID, name))
	clear(key[:])
	return plaintext, err
}

func deviceCachePurpose(name string) string {
	return "device-cache/" + name
}
