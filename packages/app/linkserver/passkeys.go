package linkserver

import (
	"context"

	"github.com/dortanes/ravenpass/packages/app/linkproto"
)

// passkeyOptions answers a passkeys request; empty lists are sent as [], never null.
func (s *Server) passkeyOptions(request linkproto.Request) (linkproto.Response, bool, error) {
	if !linkproto.ValidOrigin(request.Origin) {
		return linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidOrigin}, false, nil
	}
	query, valid := request.PasskeyQuery()
	if !valid {
		return linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidRequest}, false, nil
	}
	if query.Mode == linkproto.PasskeyCreate {
		targets, err := s.vault.PasskeyTargets(query)
		if err != nil {
			return refusal(request, err)
		}
		targets.Targets = append([]linkproto.PasskeyTarget{}, targets.Targets...)
		return linkproto.Response{ID: request.ID, Result: targets}, false, nil
	}
	passkeys, err := s.vault.Passkeys(query)
	if err != nil {
		return refusal(request, err)
	}
	passkeys.Passkeys = append([]linkproto.PasskeyOption{}, passkeys.Passkeys...)
	return linkproto.Response{ID: request.ID, Result: passkeys}, false, nil
}

// createPasskey answers a passkey-create request.
func (s *Server) createPasskey(c *connection, transport *linkproto.Transport, request linkproto.Request) error {
	if !linkproto.ValidOrigin(request.Origin) {
		return c.reply(transport, linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidOrigin})
	}
	creation, valid := request.PasskeyCreation()
	if !valid {
		return c.reply(transport, linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidRequest})
	}
	return answerVerified(s, c, transport, request, func(ctx context.Context, asked func(linkproto.Progress)) (linkproto.CreatedPasskey, error) {
		return s.vault.CreatePasskey(ctx, creation, asked)
	})
}

// signPasskey answers a passkey-sign request.
func (s *Server) signPasskey(c *connection, transport *linkproto.Transport, request linkproto.Request) error {
	if !linkproto.ValidOrigin(request.Origin) {
		return c.reply(transport, linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidOrigin})
	}
	signIn, valid := request.PasskeySignIn()
	if !valid {
		return c.reply(transport, linkproto.Response{ID: request.ID, Error: linkproto.ErrorInvalidRequest})
	}
	return answerVerified(s, c, transport, request, func(ctx context.Context, asked func(linkproto.Progress)) (linkproto.PasskeyAssertion, error) {
		return s.vault.SignPasskey(ctx, signIn, asked)
	})
}
