// Package noosisfuncs defines an [analysis.Analyzer] that forbids the legacy
// os error predicates os.IsExist, os.IsNotExist, os.IsPermission and
// os.IsTimeout.
//
// These functions predate errors.Is and do not unwrap errors wrapped with
// fmt.Errorf's %w verb, so they silently answer false for wrapped errors.
// Their documentation directs new code to errors.Is with the corresponding
// sentinel, and this analyzer reports each use and suggests that form:
//
//	os.IsExist(err)      → errors.Is(err, fs.ErrExist)
//	os.IsNotExist(err)   → errors.Is(err, fs.ErrNotExist)
//	os.IsPermission(err) → errors.Is(err, fs.ErrPermission)
//	os.IsTimeout(err)    → errors.Is(err, os.ErrDeadlineExceeded),
//	                       or another sentinel appropriate to the call
//
// The io/fs sentinels are the spelling the os documentation recommends;
// os.ErrExist, os.ErrNotExist and os.ErrPermission are aliases of them.
// os.IsTimeout has no io/fs counterpart, so its suggestion stays in package
// os.
//
// Each predicate is reported only when the analyzed module targets a Go
// version in which its replacement exists: Go 1.16 (package io/fs) for the
// first three, Go 1.15 (os.ErrDeadlineExceeded) for os.IsTimeout. Below Go
// 1.15 the analyzer does nothing.
//
// An autofix rewrites a direct call to the errors.Is form, adding the
// "errors" and "io/fs" imports when needed. os.IsTimeout gets no autofix —
// which sentinel expresses "timeout" depends on the call that produced the
// error — and neither does a predicate passed around as a function value. The
// rewrite is not strictly behavior-preserving: errors.Is also matches wrapped
// errors, which is exactly what the predicates fail to do.
//
// To keep a deliberate use, annotate its line — or the line directly above —
// with an //ngignore:noosisfuncs directive:
//
//	if os.IsNotExist(err) { //ngignore:noosisfuncs matching unwrapped errors only
//
// The directive form is `//ngignore:<name>[,<name>...] [reason]`. The analyzer
// honors it itself, so suppression works under every run mode — the standalone
// driver, `go vet -vettool`, and editor/hook integrations — not only under
// nolint-aware runners. Generated files (see go/ast.IsGenerated) are skipped.
package noosisfuncs

import (
	"fmt"
	"go/ast"
	"go/types"
	"path"
	"strings"

	"github.com/ngicks/go-ngcheckers/internal/directive"
	"github.com/ngicks/go-ngcheckers/internal/generated"
	"github.com/ngicks/go-ngcheckers/internal/gover"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Doc documents the analyzer; it is surfaced by `ngcheckers help noosisfuncs`.
const Doc = `forbid the legacy os error predicates (os.IsNotExist etc.)

os.IsExist, os.IsNotExist, os.IsPermission and os.IsTimeout predate errors.Is
and do not unwrap errors wrapped with fmt.Errorf's %w verb. This analyzer
reports each use and suggests the errors.Is replacement their documentation
directs new code to:

	os.IsExist(err)      -> errors.Is(err, fs.ErrExist)
	os.IsNotExist(err)   -> errors.Is(err, fs.ErrNotExist)
	os.IsPermission(err) -> errors.Is(err, fs.ErrPermission)
	os.IsTimeout(err)    -> errors.Is with a sentinel appropriate to the
	                        call, such as os.ErrDeadlineExceeded

A predicate is reported only when the analyzed module targets a Go version in
which its replacement exists: Go 1.16 (package io/fs) for the first three, Go
1.15 (os.ErrDeadlineExceeded) for os.IsTimeout. Direct calls are autofixed,
except os.IsTimeout, whose right sentinel depends on the call. Generated files
(those with a "// Code generated ... DO NOT EDIT." marker) are skipped. To
keep a deliberate use, annotate its line (or the line above) with
//ngignore:noosisfuncs <reason>. This directive is honored directly, so it
works under go vet and the standalone driver.`

// Analyzer is the noosisfuncs analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "noosisfuncs",
	Doc:      Doc,
	URL:      "https://github.com/ngicks/go-ngcheckers/tree/main/rules/noosisfuncs",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// target describes one forbidden os predicate.
type target struct {
	// sentinelPkg is the import path of the package holding the sentinel
	// error to match with errors.Is, and sentinel its name there. The os
	// documentation recommends the io/fs sentinels where they exist.
	sentinelPkg string
	sentinel    string
	// minVersion is the lowest Go version in which the errors.Is replacement
	// exists; below it the predicate is not reported.
	minVersion string
	// exact records whether errors.Is(err, <sentinelPkg>.<sentinel>) is the
	// documented drop-in replacement, enabling the autofix. os.IsTimeout has
	// no single equivalent sentinel, so it is reported without a fix.
	exact bool
}

var targets = map[string]target{
	"IsExist": {sentinelPkg: "io/fs", sentinel: "ErrExist", minVersion: "go1.16", exact: true},
	"IsNotExist": {
		sentinelPkg: "io/fs",
		sentinel:    "ErrNotExist",
		minVersion:  "go1.16",
		exact:       true,
	},
	"IsPermission": {
		sentinelPkg: "io/fs",
		sentinel:    "ErrPermission",
		minVersion:  "go1.16",
		exact:       true,
	},
	"IsTimeout": {
		sentinelPkg: "os",
		sentinel:    "ErrDeadlineExceeded",
		minVersion:  "go1.15",
		exact:       false,
	},
}

func run(pass *analysis.Pass) (any, error) {
	if !gover.AtLeast(pass, "go1.15") {
		// Go 1.15 is the lowest minVersion among the targets; below it no
		// predicate has a recommendable replacement. Do nothing and return.
		return nil, nil
	}

	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	// Generated files are not edited by hand, so reporting predicates in them
	// only adds noise. Skip nodes that live in one.
	gen := generated.Collect(pass.Fset, pass.Files)

	insp.WithStack(
		[]ast.Node{(*ast.SelectorExpr)(nil)},
		func(n ast.Node, push bool, stack []ast.Node) bool {
			if !push {
				return false
			}
			sel := n.(*ast.SelectorExpr)
			tgt, ok := targets[sel.Sel.Name]
			if !ok || !isOSFunc(pass, sel) {
				return true
			}
			if !gover.AtLeast(pass, tgt.minVersion) {
				return true
			}
			if gen.Contains(sel.Pos()) {
				return true
			}
			file, _ := stack[0].(*ast.File)
			if directive.SuppressesLine(pass.Fset, file, pass.Analyzer.Name, sel.Pos()) {
				return true
			}

			diag := analysis.Diagnostic{
				Pos:     sel.Pos(),
				End:     sel.End(),
				Message: message(sel.Sel.Name, tgt),
			}
			if fix, ok := suggestedFix(pass, file, stack, sel, tgt); ok {
				diag.SuggestedFixes = []analysis.SuggestedFix{fix}
			}
			pass.Report(diag)
			return true
		},
	)

	return nil, nil
}

// isOSFunc reports whether sel selects a function of the standard os package.
// It prefers type information — which also sees through aliased imports and
// rejects same-named methods or locals — but falls back to a syntactic check
// so it still works under syntax-only load modes.
func isOSFunc(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if pass.TypesInfo != nil {
		fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
		return ok && fn.Pkg() != nil && fn.Pkg().Path() == "os"
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "os"
}

// message renders the diagnostic text for one predicate. The sentinel is
// spelled through its package's default name (fs.ErrNotExist, ...).
func message(name string, tgt target) string {
	ver := strings.TrimPrefix(tgt.minVersion, "go")
	sentinel := path.Base(tgt.sentinelPkg) + "." + tgt.sentinel
	use := fmt.Sprintf("errors.Is(err, %s) (Go %s+)", sentinel, ver)
	if !tgt.exact {
		use = fmt.Sprintf(
			"errors.Is with a sentinel appropriate to the call, such as %s (Go %s+)",
			sentinel,
			ver,
		)
	}
	return fmt.Sprintf("avoid os.%s, which does not unwrap errors; use %s instead, "+
		"or add //ngignore:noosisfuncs <reason> if os.%s is intended here", name, use, name)
}
