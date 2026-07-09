package a

import goos "os"

// aliased references the os package through an import alias; the predicate is
// still detected. The file's single non-grouped import also exercises adding
// "errors" and "io/fs" as their own declarations; the alias import itself
// becomes unused once the call is rewritten, and fix application drops it.
func aliased(err error) bool {
	return goos.IsNotExist(err) // want `avoid os\.IsNotExist`
}
