package api

import (
	"time"

	"github.com/dortanes/ravenpass/packages/vault"
)

// OneTimeCode is the code a credential's setup produces now.
type OneTimeCode struct {
	Code   string `json:"code"`
	Digits int    `json:"digits"`
	Period int    `json:"period"`
	// ExpiresAt is in Unix milliseconds.
	ExpiresAt int64 `json:"expiresAt"`
}

// GenerateOneTimeCode derives the current code from setup without reading or keeping vault data.
func (s *Service) GenerateOneTimeCode(setup string) (OneTimeCode, error) {
	code, err := vault.GenerateOneTimeCode(setup, time.Now())
	if err != nil {
		return OneTimeCode{}, present(err)
	}
	return OneTimeCode{
		Code:      code.Code,
		Digits:    code.Digits,
		Period:    code.Period,
		ExpiresAt: code.ExpiresAt.UnixMilli(),
	}, nil
}
