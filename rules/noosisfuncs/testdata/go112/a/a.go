// Package a targets Go 1.12, below every predicate's replacement (io/fs is
// Go 1.16, os.ErrDeadlineExceeded is Go 1.15). The analyzer must be a no-op
// here, so there are no "want" comments: any diagnostic would fail the test.
package a

import "os"

func f(err error) bool {
	return os.IsNotExist(err) || os.IsTimeout(err)
}
