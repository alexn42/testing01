// Package apperr marks expected, user-facing failures (a bad path, an
// unsupported format, a missing external tool) as distinct from bugs, so the
// CLI can report them as a clean JSON error instead of a stack trace, while
// letting anything else propagate normally.
package apperr

import "fmt"

type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func New(format string, args ...interface{}) error {
	return &Error{msg: fmt.Sprintf(format, args...)}
}
