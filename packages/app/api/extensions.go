package api

import (
	"context"

	"github.com/dortanes/ravenpass/packages/app/linkserver"
	"github.com/dortanes/ravenpass/packages/app/linkstore"
)

// ExtensionLinker is the link server the browser extension reaches this app through.
type ExtensionLinker interface {
	Extensions() ([]linkstore.Extension, error)
	Reachable() bool
	Begin() (linkserver.Offer, error)
	Await(ctx context.Context) (linkstore.Extension, error)
	Cancel()
	Waiting() (linkserver.Offer, bool)
	Unlink(id string) error
	Rename(id, name string) error
}

// noExtensions is the linker of a host browser extensions cannot reach.
type noExtensions struct{}

func (noExtensions) Extensions() ([]linkstore.Extension, error) { return nil, nil }
func (noExtensions) Reachable() bool                            { return false }
func (noExtensions) Begin() (linkserver.Offer, error) {
	return linkserver.Offer{}, linkserver.ErrUnavailable
}
func (noExtensions) Await(context.Context) (linkstore.Extension, error) {
	return linkstore.Extension{}, linkserver.ErrUnavailable
}
func (noExtensions) Cancel()                           {}
func (noExtensions) Waiting() (linkserver.Offer, bool) { return linkserver.Offer{}, false }
func (noExtensions) Unlink(string) error               { return linkstore.ErrNotFound }
func (noExtensions) Rename(string, string) error       { return linkstore.ErrNotFound }

// LinkedExtension is a browser extension linked to this app.
type LinkedExtension struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// LinkedAt is in Unix milliseconds.
	LinkedAt int64 `json:"linkedAt"`
}

// ExtensionLinks lists the linked extensions, and whether they can reach this app.
type ExtensionLinks struct {
	Extensions []LinkedExtension `json:"extensions"`
	Reachable  bool              `json:"reachable"`
}

// ExtensionLinkOffer is a one-time connection key waiting for an extension.
type ExtensionLinkOffer struct {
	Key string `json:"key"`
	// ExpiresAt is in Unix milliseconds.
	ExpiresAt int64 `json:"expiresAt"`
}

// ExtensionLinks lists the linked extensions and whether they can reach this app.
func (s *Service) ExtensionLinks() (ExtensionLinks, error) {
	extensions, err := s.links.Extensions()
	if err != nil {
		return ExtensionLinks{}, present(err)
	}
	linked := make([]LinkedExtension, len(extensions))
	for i, extension := range extensions {
		linked[i] = presentExtension(extension)
	}
	return ExtensionLinks{Extensions: linked, Reachable: s.links.Reachable()}, nil
}

// BeginExtensionLink creates a one-time connection key, replacing the one waiting.
func (s *Service) BeginExtensionLink() (ExtensionLinkOffer, error) {
	offer, err := s.links.Begin()
	if err != nil {
		return ExtensionLinkOffer{}, present(err)
	}
	return ExtensionLinkOffer{Key: offer.Key, ExpiresAt: offer.ExpiresAt.UnixMilli()}, nil
}

// AwaitExtensionLink returns the extension that links with the waiting key; canceling the call cancels the key.
func (s *Service) AwaitExtensionLink(ctx context.Context) (LinkedExtension, error) {
	extension, err := s.links.Await(ctx)
	if err != nil {
		return LinkedExtension{}, present(err)
	}
	return presentExtension(extension), nil
}

// CancelExtensionLink withdraws the waiting connection key.
func (s *Service) CancelExtensionLink() error {
	s.links.Cancel()
	return nil
}

// CopyExtensionLinkKey copies the waiting key until it is spent, expires, or is canceled or replaced.
func (s *Service) CopyExtensionLinkKey() error {
	return s.copyExtensionLinkKey(clearWhenClosed)
}

func (s *Service) copyExtensionLinkKey(watch func(<-chan struct{}, func())) error {
	offer, waiting := s.links.Waiting()
	if !waiting {
		return fail(failureLinkExpired)
	}
	if !s.copyText(offer.Key, whenClosed(offer.Done, watch)) {
		return fail(failureCopyFailed)
	}
	return nil
}

// UnlinkExtension removes the linked extension id.
func (s *Service) UnlinkExtension(id string) error {
	return present(s.links.Unlink(id))
}

// RenameExtension names the linked extension id anew, trimmed; an empty or too long name is refused.
func (s *Service) RenameExtension(id, name string) error {
	return present(s.links.Rename(id, name))
}

func presentExtension(extension linkstore.Extension) LinkedExtension {
	return LinkedExtension{ID: extension.ID, Name: extension.Name, LinkedAt: extension.LinkedAt.UnixMilli()}
}
