package api

import (
	"errors"
	"time"

	"github.com/dortanes/ravenpass/packages/app/genhistory"
)

// GeneratorOptions are what the password generator makes.
type GeneratorOptions = genhistory.Options

// GeneratedPassword is one value the generator made; At is RFC 3339 in UTC.
type GeneratedPassword struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
	At    string `json:"at"`
}

// GeneratorState is the open vault's generator history, newest first, and the options last used.
type GeneratorState struct {
	Options GeneratorOptions    `json:"options"`
	History []GeneratedPassword `json:"history"`
}

// GeneratorState reads the open vault's generator history and options.
func (s *Service) GeneratorState() (GeneratorState, error) {
	if s.generator == nil {
		return GeneratorState{}, fail(failureGeneral)
	}
	state, err := s.generator.State()
	if err != nil {
		return GeneratorState{}, presentGenerator(err)
	}
	history := make([]GeneratedPassword, len(state.History))
	for i, entry := range state.History {
		history[i] = generated(entry)
	}
	return GeneratorState{Options: state.Options, History: history}, nil
}

// GeneratePassword makes a password or passphrase with options and records it in the history.
func (s *Service) GeneratePassword(options GeneratorOptions) (GeneratedPassword, error) {
	if s.generator == nil {
		return GeneratedPassword{}, fail(failureGeneral)
	}
	entry, err := s.generator.Generate(options)
	if err != nil {
		return GeneratedPassword{}, presentGenerator(err)
	}
	return generated(entry), nil
}

// ClearGeneratorHistory forgets every generated value of the open vault.
func (s *Service) ClearGeneratorHistory() error {
	if s.generator == nil {
		return fail(failureGeneral)
	}
	return presentGenerator(s.generator.Clear())
}

// CopyGeneratedPassword puts a generated value on the clipboard, cleared like any other copied secret.
func (s *Service) CopyGeneratedPassword(value string) error {
	if value == "" {
		return fail(failureFieldEmpty)
	}
	if !s.copyToClipboard(value, clearAfter) {
		return fail(failureCopyFailed)
	}
	return nil
}

func generated(entry genhistory.Entry) GeneratedPassword {
	return GeneratedPassword{Value: entry.Value, Kind: entry.Kind, At: entry.At.UTC().Format(time.RFC3339)}
}

func presentGenerator(err error) error {
	if errors.Is(err, genhistory.ErrInvalidOptions) {
		return fail(failureInvalidItem)
	}
	return present(err)
}
