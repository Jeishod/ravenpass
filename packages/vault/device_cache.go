package vault

import "golang.org/x/crypto/chacha20poly1305"

const maxDeviceCacheBytes = 32 << 20

// deviceKind keeps device caches and device data apart: each has its own key purpose and additional data tag.
type deviceKind struct {
	purpose string
	tag     byte
}

var (
	deviceCache = deviceKind{purpose: "device-cache/", tag: 5}
	deviceData  = deviceKind{purpose: "device-data/", tag: 6}
)

// SealDeviceCache seals device-side data that opens only in this vault under the same name, a fixed purpose and never user data.
func (s *Session) SealDeviceCache(name string, plaintext []byte) ([]byte, error) {
	return s.sealOnDevice(deviceCache, name, plaintext)
}

// OpenDeviceCache opens what SealDeviceCache sealed for the same name in this vault.
func (s *Session) OpenDeviceCache(name string, sealed []byte) ([]byte, error) {
	return s.openOnDevice(deviceCache, name, sealed)
}

func (s *Session) sealOnDevice(kind deviceKind, name string, plaintext []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, ErrLocked
	}
	return sealOnDevice(&s.dataKey, s.vaultID, kind, name, plaintext)
}

func (s *Session) openOnDevice(kind deviceKind, name string, sealed []byte) ([]byte, error) {
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
	key, err := deriveKey(s.dataKey[:], s.vaultID, kind.purpose+name)
	if err != nil {
		return nil, err
	}
	plaintext, err := openBox(key, box, kind.aad(s.vaultID, name))
	clear(key[:])
	return plaintext, err
}

func sealOnDevice(dataKey *[32]byte, vaultID ID, kind deviceKind, name string, plaintext []byte) ([]byte, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	if len(plaintext) > maxDeviceCacheBytes-chacha20poly1305.Overhead {
		return nil, ErrResourceLimit
	}
	key, err := deriveKey(dataKey[:], vaultID, kind.purpose+name)
	if err != nil {
		return nil, err
	}
	box, err := seal(key, plaintext, kind.aad(vaultID, name))
	clear(key[:])
	if err != nil {
		return nil, err
	}
	return encodeEnvelope(vaultID, box), nil
}

func (k deviceKind) aad(vaultID ID, name string) []byte {
	hash := headerHash(vaultID)
	result := make([]byte, 0, 33+len(name))
	result = append(result, k.tag)
	result = append(result, hash[:]...)
	return append(result, name...)
}
