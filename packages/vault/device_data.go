package vault

// SealDeviceData seals the owner's data kept on this device, such as generated passwords, so it opens only in this
// vault under the same name. Unlike a device cache it is user data: its holder keeps it when it fails to open, and a
// key change seals it again with Rekey.SealDeviceData.
func (s *Session) SealDeviceData(name string, plaintext []byte) ([]byte, error) {
	return s.sealOnDevice(deviceData, name, plaintext)
}

// OpenDeviceData opens what SealDeviceData sealed for the same name in this vault.
func (s *Session) OpenDeviceData(name string, sealed []byte) ([]byte, error) {
	return s.openOnDevice(deviceData, name, sealed)
}

// SealDeviceData seals device data under the new vault key, which OpenDeviceData opens once the rekey commits.
func (r *Rekey) SealDeviceData(name string, plaintext []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.discarded {
		return nil, ErrLocked
	}
	return sealOnDevice(&r.key.data, r.vaultID, deviceData, name, plaintext)
}
