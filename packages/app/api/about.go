package api

import (
	"errors"
	"io/fs"
	"strings"
)

// About is what the host says of the running app and its system.
type About interface {
	// System names the operating system and its version; the version is empty where the system does not report it.
	System() (name, version string)
	// Notices returns the build's third-party notices; fs.ErrNotExist reports a build that ships none.
	Notices() ([]byte, error)
}

// SupportDetails are the technical facts a problem report carries; none names the device, its owner or a vault.
type SupportDetails struct {
	Platform  string `json:"platform"`
	OSVersion string `json:"osVersion"`
}

// SupportDetails reports the operating system and its version.
func (s *Service) SupportDetails() (SupportDetails, error) {
	platform, version := s.about.System()
	return SupportDetails{Platform: platform, OSVersion: version}, nil
}

// ThirdPartyNotices returns the license text the build ships, empty when it ships none.
func (s *Service) ThirdPartyNotices() (string, error) {
	text, err := s.about.Notices()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", present(err)
	}
	return strings.ToValidUTF8(string(text), "�"), nil
}
