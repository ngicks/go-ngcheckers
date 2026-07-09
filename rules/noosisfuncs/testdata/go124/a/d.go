package a

import "os"

// shadowed has the name "errors" taken by a parameter, so no clean errors.Is
// rewrite exists at the call site; the use is reported without a fix.
func shadowed(err error, errors int) bool {
	_ = errors
	return os.IsExist(err) // want `avoid os\.IsExist`
}
