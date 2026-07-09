// Package gover determines the Go language version targeted by an analyzed
// package, so version-gated checkers can decide whether their recommendation
// is available there.
package gover

import (
	goversion "go/version"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// AtLeast reports whether the analyzed package targets Go version min (a
// canonical "go1.N" string) or newer. When the version cannot be determined it
// errs toward true — modern toolchains are well past any version a checker
// gates on, and real builds (go vet, go/packages) always supply module
// metadata.
func AtLeast(pass *analysis.Pass, min string) bool {
	v := Effective(pass)
	if v == "" {
		return true
	}
	return goversion.Compare(v, min) >= 0
}

// Effective returns the Go language version targeted by the analyzed package
// as a canonical "go1.N" string, or "" when it is unknown. The type-checker's
// version (derived from the module's go directive) is the most reliable source
// and is available under every loader; the module metadata is a fallback.
func Effective(pass *analysis.Pass) string {
	if pass.Pkg != nil {
		if v := normalize(pass.Pkg.GoVersion()); v != "" {
			return v
		}
	}
	if pass.Module != nil {
		if v := normalize(pass.Module.GoVersion); v != "" {
			return v
		}
	}
	return ""
}

// normalize canonicalizes a Go version string to the "go1.N" form expected by
// go/version, returning "" when the input is empty or invalid. It accepts both
// "1.24" (go.mod go directive form) and "go1.24" (go/types form).
func normalize(v string) string {
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "go") {
		v = "go" + v
	}
	if !goversion.IsValid(v) {
		return ""
	}
	return v
}
