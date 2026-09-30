// Package apperr is the error vocabulary an application's services share with
// its transports. Services return these errors, wrapped with a reason; each
// transport maps them to a response in one place (ginhelpers.Status for gin
// pages and JSON APIs). The package imports nothing, so services can use it
// without depending on a transport.
package apperr

import "errors"

var (
	// ErrNotFound: the thing doesn't exist, or the actor may not know it
	// exists.
	ErrNotFound = errors.New("not found")
	// ErrForbidden: the actor can see the thing but may not do this to it.
	ErrForbidden = errors.New("forbidden")
	// ErrNeedsLogin: an anonymous actor would be allowed after logging in.
	ErrNeedsLogin = errors.New("needs login")
	// ErrConflict: the change collides with existing state. Wrap it with the
	// reason: fmt.Errorf("%w: username is taken", apperr.ErrConflict).
	ErrConflict = errors.New("conflict")
)

// ValidationError rejects an input. Message is shown to the user as is, so it
// reads as a sentence. Field names the input field, or is empty when the
// error is about the input as a whole.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Invalid returns a ValidationError for field.
func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
