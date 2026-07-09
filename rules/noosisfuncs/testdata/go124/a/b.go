package a

import (
	"os"
)

// missingImport lives in a file with neither an "errors" nor an "io/fs"
// import; the fix must insert both into the import block.
func missingImport(name string) bool {
	_, err := os.Stat(name)
	return os.IsPermission(err) // want `avoid os\.IsPermission`
}
