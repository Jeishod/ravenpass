package api

import "github.com/dortanes/ravenpass/packages/app/siteicons"

// SiteIcons reports whether website icons load.
type SiteIcons struct {
	Enabled bool `json:"enabled"`
}

// SiteIcon is a site's icon.
type SiteIcon struct {
	// Image is a base64 PNG, empty for none.
	Image string `json:"image"`
	// Tint is the main colour as "#rrggbb", empty for a grey icon or none.
	Tint string `json:"tint"`
}

// GetSiteIcons reports whether website icons load.
func (s *Service) GetSiteIcons() (SiteIcons, error) {
	return SiteIcons{Enabled: s.preferences.SiteIcons()}, nil
}

// SetSiteIcons records the choice; turning icons off stops fetches and deletes every cached icon.
func (s *Service) SetSiteIcons(enabled bool) error {
	if err := s.preferences.SetSiteIcons(enabled); err != nil {
		return present(err)
	}
	return present(s.icons.SetEnabled(enabled))
}

// SiteIcon returns a site's icon, fetching only sites a credential or card of the open vault names.
func (s *Service) SiteIcon(site string) (SiteIcon, error) {
	named, err := s.vault.NamesSite(site)
	if err != nil {
		return SiteIcon{}, present(err)
	}
	if !named {
		return SiteIcon{}, nil
	}
	icon, err := s.icons.Icon(site)
	if err != nil {
		return SiteIcon{}, present(err)
	}
	return SiteIcon{Image: icon, Tint: siteicons.Tint(icon)}, nil
}
