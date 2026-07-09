// Package a targets Go 1.15: os.ErrDeadlineExceeded exists, so os.IsTimeout
// is reported, but package io/fs does not exist until Go 1.16, so the other
// predicates are not.
package a

import "os"

func f(err error) {
	_ = os.IsNotExist(err)
	_ = os.IsTimeout(err) // want `avoid os\.IsTimeout`
}
