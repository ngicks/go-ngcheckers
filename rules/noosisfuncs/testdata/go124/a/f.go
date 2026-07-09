package a

import "os"

// fsTaken has the name "fs" taken by a local at the call site while io/fs is
// not imported, so no clean sentinel reference exists; the use is reported
// without a fix.
func fsTaken(err error) bool {
	fs := "not the package"
	_ = fs
	return os.IsNotExist(err) // want `avoid os\.IsNotExist`
}
