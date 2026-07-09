// Package a exercises the noosisfuncs analyzer on a module targeting Go 1.24,
// where every errors.Is replacement is available.
package a

import (
	"errors"
	"fmt"
	"os"
)

// direct covers plain calls of each predicate; all are reported and, except
// os.IsTimeout, autofixed to errors.Is.
func direct(err error) {
	if os.IsNotExist(err) { // want `avoid os\.IsNotExist`
		_ = err
	}
	_ = os.IsExist(err)      // want `avoid os\.IsExist`
	_ = os.IsPermission(err) // want `avoid os\.IsPermission`
	_ = os.IsTimeout(err)    // want `avoid os\.IsTimeout`
}

// nested covers a call inside a larger expression and an arbitrary expression
// as the argument; the argument text must survive the rewrite verbatim.
func nested(err error) bool {
	return !os.IsNotExist(err) && // want `avoid os\.IsNotExist`
		os.IsExist(fmt.Errorf("wrap: %w", err)) // want `avoid os\.IsExist`
}

// value passes a predicate around as a function value; it is reported but
// there is no call to rewrite, so no fix is offered.
func value() func(error) bool {
	return os.IsNotExist // want `avoid os\.IsNotExist`
}

// already is the recommended form and is not reported.
func already(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// suppressed keeps deliberate uses via //ngignore directives; none of these
// may be reported, and -fix must leave them untouched.
func suppressed(err error) {
	if os.IsNotExist(err) { //ngignore:noosisfuncs matching unwrapped errors only
		_ = err
	}
	//ngignore:noosisfuncs reason text is free-form
	_ = os.IsExist(err)

	// A comma-separated list that names this checker among others suppresses
	// too, and a directive with no trailing reason still suppresses.
	_ = os.IsPermission(err) //ngignore:othercheck,noosisfuncs
	// A directive naming only a different checker does NOT suppress this one.
	//ngignore:othercheck unrelated
	_ = os.IsExist(err) // want `avoid os\.IsExist`
}

// IsNotExist is a package-local function whose name collides with the os
// predicate; calling it unqualified is fine.
func IsNotExist(error) bool { return false }

// notTheOSPackage covers lookalikes that must NOT be reported: the local
// function above, and a method on a variable that shadows the os package name.
func notTheOSPackage(err error) {
	_ = IsNotExist(err)
	var os shadow
	_ = os.IsNotExist(err)
}

type shadow struct{}

func (shadow) IsNotExist(error) bool { return false }
