package model

import (
	"errors"
	"fmt"
	"strconv"
)

// Why an entity could not be used as asked. These are the only causes the API maps onto a status
// code, so a new one has to be mapped there deliberately rather than falling through unnoticed.
var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
	ErrInUse         = errors.New("is in use")
)

// entityError names the entity a request could not act on and why. It carries the name so no
// layer above has to restate it, which is what let a reported status and its message drift apart:
// every caller now renders the same sentence, `routine "daily" not found`, from the error itself.
// The cause stays reachable through errors.Is.
type entityError struct {
	// Kind is the entity as the API names it: routine, storage, cluster, policy, job.
	Kind string
	// Name identifies the entity. Numeric identifiers, such as restore job IDs, print bare.
	Name any
	// Cause is one of ErrNotFound, ErrAlreadyExists or ErrInUse.
	Cause error
	// Detail is an optional clause appended after a colon, such as the routine holding a reference.
	Detail string
}

func (e *entityError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s %s %s", e.Kind, quoteName(e.Name), e.Cause)
	}

	return fmt.Sprintf("%s %s %s: %s", e.Kind, quoteName(e.Name), e.Cause, e.Detail)
}

func (e *entityError) Unwrap() error {
	return e.Cause
}

func NotFound(kind string, name any) error {
	return &entityError{Kind: kind, Name: name, Cause: ErrNotFound}
}

func AlreadyExists(kind string, name any) error {
	return &entityError{Kind: kind, Name: name, Cause: ErrAlreadyExists}
}

func InUse(kind string, name any, detail string) error {
	return &entityError{Kind: kind, Name: name, Cause: ErrInUse, Detail: detail}
}

// quoteName quotes string names and prints other identifiers, such as numeric job IDs, as they
// are; %q would render an integer as a rune literal.
func quoteName(name any) string {
	if s, ok := name.(string); ok {
		return strconv.Quote(s)
	}

	return fmt.Sprint(name)
}
