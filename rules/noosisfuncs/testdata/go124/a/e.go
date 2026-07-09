package a

import (
	"os"

	fsx "io/fs"
)

// aliasedFS references io/fs under an import alias; the sentinel in the fix
// is spelled through the same alias.
func aliasedFS(err error) (bool, error) {
	if os.IsNotExist(err) { // want `avoid os\.IsNotExist`
		return true, nil
	}
	return false, &fsx.PathError{Op: "stat", Path: "x", Err: err}
}
