// Suggested-fix construction for the noosisfuncs analyzer: rewriting a direct
// os.IsX(err) call to errors.Is(err, fs.ErrX), including referencing — and if
// necessary importing — the errors and io/fs packages at the call site.

package noosisfuncs

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// suggestedFix builds the errors.Is rewrite for a direct call os.IsX(err).
// ok=false when there is nothing to safely rewrite: the predicate is not a
// drop-in (os.IsTimeout), the reference is not a one-argument call (e.g. the
// function is passed as a value), or the errors or sentinel package cannot be
// referenced cleanly at the call site.
func suggestedFix(
	pass *analysis.Pass,
	file *ast.File,
	stack []ast.Node,
	sel *ast.SelectorExpr,
	tgt target,
) (analysis.SuggestedFix, bool) {
	if !tgt.exact || len(stack) < 2 {
		return analysis.SuggestedFix{}, false
	}
	call, ok := stack[len(stack)-2].(*ast.CallExpr)
	if !ok || call.Fun != sel || len(call.Args) != 1 {
		return analysis.SuggestedFix{}, false
	}
	errorsQual, errorsMissing, ok := pkgQualifier(pass, file, call.Pos(), "errors")
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	sentinelQual, sentinelMissing, ok := pkgQualifier(pass, file, call.Pos(), tgt.sentinelPkg)
	if !ok {
		return analysis.SuggestedFix{}, false
	}

	// Emit edits in position order: one insertion covering every missing
	// import (drivers coalesce it when several diagnostics in the file add
	// the same one), then the two call-site edits. Rewriting around the
	// argument leaves its source text — arbitrary expressions included —
	// exactly as written.
	var edits []analysis.TextEdit
	var missing []string
	if errorsMissing {
		missing = append(missing, "errors")
	}
	if sentinelMissing {
		missing = append(missing, tgt.sentinelPkg)
	}
	if len(missing) > 0 {
		edits = append(edits, importsEdit(file, missing))
	}
	arg := call.Args[0]
	edits = append(
		edits,
		// "os.IsX(" → "errors.Is("
		analysis.TextEdit{Pos: call.Pos(), End: arg.Pos(), NewText: []byte(errorsQual + ".Is(")},
		// ")" (and any trailing comma) → ", fs.ErrX)"
		analysis.TextEdit{
			Pos:     arg.End(),
			End:     call.End(),
			NewText: []byte(", " + sentinelQual + "." + tgt.sentinel + ")"),
		},
	)
	return analysis.SuggestedFix{
		Message: fmt.Sprintf(
			"Replace os.%s with errors.Is(err, %s.%s)",
			sel.Sel.Name,
			path.Base(tgt.sentinelPkg),
			tgt.sentinel,
		),
		TextEdits: edits,
	}, true
}

// pkgQualifier decides how to reference the package with the given import
// path at pos in file. It returns the qualifier to spell references with (the
// package's own name or the local import alias) and, when the package is not
// yet imported, missing=true — the caller then adds the import via
// [importsEdit]. ok=false when no clean reference exists: the name is
// shadowed or claimed by something else at pos, or the package is blank- or
// dot-imported.
func pkgQualifier(
	pass *analysis.Pass,
	file *ast.File,
	pos token.Pos,
	pkgPath string,
) (qual string, missing bool, ok bool) {
	name := ""
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err != nil || p != pkgPath {
			continue
		}
		name = path.Base(pkgPath)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		break
	}
	switch name {
	case "":
		// Not imported. Claim the package's own name only when nothing else
		// — a local variable, another package imported under that name —
		// resolves to it at pos.
		name = path.Base(pkgPath)
		if obj, known := lookupAt(pass, file, name, pos); known && obj != nil {
			return "", false, false
		}
		return name, true, true
	case "_", ".":
		// A blank import gives no name to spell references with, and after a
		// dot import they would be bare identifiers — too easy to collide.
		// Report without a fix.
		return "", false, false
	default:
		// Imported, possibly aliased. Make sure the name still resolves to
		// the package at pos rather than to a shadowing local.
		if obj, known := lookupAt(pass, file, name, pos); known {
			if _, isPkg := obj.(*types.PkgName); !isPkg {
				return "", false, false
			}
		}
		return name, false, true
	}
}

// lookupAt resolves name at pos within file, position-aware (a local declared
// after pos does not count). known=false when type information is unavailable
// and the caller should fall back to syntax-derived knowledge.
func lookupAt(
	pass *analysis.Pass,
	file *ast.File,
	name string,
	pos token.Pos,
) (obj types.Object, known bool) {
	if pass.TypesInfo == nil {
		return nil, false
	}
	scope := pass.TypesInfo.Scopes[file]
	if scope == nil {
		return nil, false
	}
	if inner := scope.Innermost(pos); inner != nil {
		scope = inner
	}
	_, obj = scope.LookupParent(name, pos)
	return obj, true
}

// importsEdit builds one TextEdit inserting imports of the given paths into
// file: into the first import block when there is one, as their own
// declarations otherwise. The insertion need not be perfectly formatted —
// gofmt output is the responsibility of whoever applies the fix.
func importsEdit(file *ast.File, paths []string) analysis.TextEdit {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = strconv.Quote(p)
	}
	for _, d := range file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if gd.Lparen.IsValid() {
			pos := gd.Lparen + 1
			return analysis.TextEdit{
				Pos:     pos,
				End:     pos,
				NewText: []byte("\n\t" + strings.Join(quoted, "\n\t")),
			}
		}
		var text strings.Builder
		for _, q := range quoted {
			text.WriteString("import " + q + "\n")
		}
		text.WriteString("\n")
		return analysis.TextEdit{Pos: gd.Pos(), End: gd.Pos(), NewText: []byte(text.String())}
	}
	// No import declaration at all; add one after the package clause.
	pos := file.Name.End()
	return analysis.TextEdit{
		Pos:     pos,
		End:     pos,
		NewText: []byte("\n\nimport " + strings.Join(quoted, "\nimport ")),
	}
}
