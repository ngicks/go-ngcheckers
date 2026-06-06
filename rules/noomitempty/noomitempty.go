// Package noomitempty defines an [analysis.Analyzer] that forbids the
// "omitempty" option in `json` struct tags.
//
// Since Go 1.24 the encoding/json package understands the "omitzero" tag
// option, which omits a field when its value is the zero value (including a
// present-but-zero value such as a zero [time.Time]) rather than when the
// value is "empty" in encoding/json's idiosyncratic sense. "omitzero" is
// almost always what callers actually want, so this analyzer flags
// "omitempty" and suggests "omitzero" instead.
//
// The analyzer does nothing when the analyzed module targets Go 1.23 or
// lower, because "omitzero" is not available there.
//
// A `json.RawMessage` field is exempt: its zero value is a nil slice, for
// which "omitempty" and "omitzero" behave identically, and "omitempty" is the
// established spelling for it.
//
// To silence a deliberate use of "omitempty", annotate the field with an
// `//ngignore:noomitempty` directive, written either on the line directly
// above the field or as a trailing comment on the field's own line:
//
//	T time.Time `json:"t,omitempty"` //ngignore:noomitempty zero is intentional
//
// The directive form is `//ngignore:<name>[,<name>...] [reason]`. The analyzer
// honors it itself, so suppression works under every run mode — the standalone
// driver, `go vet -vettool`, and editor/hook integrations — not only under
// nolint-aware runners. (golangci-lint's own `//nolint:noomitempty` also
// suppresses the diagnostic, but only when run under golangci-lint.)
package noomitempty

import (
	"go/ast"
	"go/token"
	"go/types"
	goversion "go/version"
	"reflect"
	"strconv"
	"strings"

	"github.com/ngicks/go-ngcheckers/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Doc documents the analyzer; it is surfaced by `ngcheckers help noomitempty`.
const Doc = `forbid the "omitempty" option in json struct tags

Since Go 1.24 the "omitzero" json tag option is available and is almost always
preferable to "omitempty". This analyzer reports json struct tags that use
"omitempty" and suggests replacing it with "omitzero".

The analyzer is a no-op for modules targeting Go 1.23 or earlier. json.RawMessage
fields are exempt. To allow a deliberate "omitempty", annotate the field with
//ngignore:noomitempty <reason> (on the field's line or the line above it). This
directive is honored directly, so it works under go vet and the standalone driver.`

// Analyzer is the noomitempty analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "noomitempty",
	Doc:      Doc,
	URL:      "https://github.com/ngicks/go-ngcheckers/tree/main/rules/noomitempty",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// minVersion is the lowest Go version that understands the "omitzero" json tag
// option. Below it the analyzer reports nothing.
const minVersion = "go1.24"

func run(pass *analysis.Pass) (any, error) {
	if !supportsOmitzero(pass) {
		// Module targets Go 1.23 or lower: "omitzero" is unavailable, so
		// there is nothing to recommend. Do nothing and return.
		return nil, nil
	}

	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	insp.Preorder([]ast.Node{(*ast.StructType)(nil)}, func(n ast.Node) {
		st := n.(*ast.StructType)
		if st.Fields == nil {
			return
		}
		for _, field := range st.Fields.List {
			checkField(pass, field)
		}
	})

	return nil, nil
}

// supportsOmitzero reports whether the analyzed package targets a Go version
// that understands the "omitzero" json tag option (Go 1.24+). When the version
// cannot be determined the analyzer errs toward running: modern toolchains are
// well past 1.24, and real builds (go vet, go/packages) always supply module
// metadata.
func supportsOmitzero(pass *analysis.Pass) bool {
	v := effectiveGoVersion(pass)
	if v == "" {
		return true
	}
	return goversion.Compare(v, minVersion) >= 0
}

// effectiveGoVersion returns the Go language version targeted by the analyzed
// package as a canonical "go1.N" string, or "" when it is unknown. The
// type-checker's version (derived from the module's go directive) is the most
// reliable source and is available under every loader; the module metadata is
// a fallback.
func effectiveGoVersion(pass *analysis.Pass) string {
	if pass.Pkg != nil {
		if v := normalizeVersion(pass.Pkg.GoVersion()); v != "" {
			return v
		}
	}
	if pass.Module != nil {
		if v := normalizeVersion(pass.Module.GoVersion); v != "" {
			return v
		}
	}
	return ""
}

// normalizeVersion canonicalizes a Go version string to the "go1.N" form
// expected by go/version, returning "" when the input is empty or invalid.
// It accepts both "1.24" (go.mod go directive form) and "go1.24" (go/types
// form).
func normalizeVersion(v string) string {
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

// message is the diagnostic text shared by the fix and no-fix report paths.
const message = `avoid "omitempty" in json struct tags; ` +
	`use "omitzero" (Go 1.24+) instead, ` +
	`or add //ngignore:noomitempty <reason> if "omitempty" is intended here`

func checkField(pass *analysis.Pass, field *ast.Field) {
	if field.Tag == nil {
		return
	}

	// Decode the Go string literal to the struct tag itself, then read the
	// json tag exactly as encoding/json does. strconv.Unquote handles both
	// raw-string (backtick) and interpreted-string (double-quoted) literals,
	// so a tag written either way is detected identically.
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return
	}
	jsonVal, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return
	}
	if _, ok := omitemptyOption(jsonVal); !ok {
		return
	}
	if isRawMessage(pass, field.Type) {
		return
	}
	if directive.Suppresses(pass.Analyzer.Name, field.Doc, field.Comment) {
		return
	}

	diag := analysis.Diagnostic{
		Pos:     field.Tag.Pos(),
		End:     field.Tag.End(),
		Message: message,
	}
	// Offer the autofix only when the "omitempty" option can be located
	// precisely in the source literal; otherwise still report the diagnostic.
	if start, end, ok := omitemptyPos(field.Tag); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: `Replace "omitempty" with "omitzero"`,
			TextEdits: []analysis.TextEdit{{
				Pos:     start,
				End:     end,
				NewText: []byte("omitzero"),
			}},
		}}
	}
	pass.Report(diag)
}

// omitemptyPos returns the source position range of the json tag's "omitempty"
// option within the tag literal, or ok=false when it cannot be located. It
// works on the raw source bytes so the range maps directly back to the file.
func omitemptyPos(tagLit *ast.BasicLit) (start, end token.Pos, ok bool) {
	raw := tagLit.Value
	if len(raw) < 2 {
		return 0, 0, false
	}
	// In an interpreted (double-quoted) literal the tag's own quote characters
	// are written as \"; in a raw (backtick) literal they are bare ". Option
	// names like "omitempty" are never escaped in either form, so once the json
	// value span is found its byte offset maps straight to the source.
	escaped := raw[0] == '"'
	content := raw[1 : len(raw)-1]
	base := tagLit.Pos() + 1 // source position of content[0]

	valStart, valEnd, ok := jsonValueSpan(content, escaped)
	if !ok {
		return 0, 0, false
	}
	off, ok := omitemptyOption(content[valStart:valEnd])
	if !ok {
		return 0, 0, false
	}
	start = base + token.Pos(valStart+off)
	return start, start + token.Pos(len("omitempty")), true
}

// jsonValueSpan locates the value of the `json` key within content (a struct
// tag literal with its outer quotes removed) and returns the byte range
// [start, end). The grammar is space-separated key:"value" pairs
// (reflect.StructTag). When escaped is true the value-delimiting quotes appear
// as the two bytes \" instead of a single ".
func jsonValueSpan(content string, escaped bool) (start, end int, ok bool) {
	n := len(content)
	// quoteAt reports whether a value-delimiting quote begins at i and, if so,
	// returns the index just past it.
	quoteAt := func(i int) (int, bool) {
		if escaped {
			if i+1 < n && content[i] == '\\' && content[i+1] == '"' {
				return i + 2, true
			}
			return i, false
		}
		if i < n && content[i] == '"' {
			return i + 1, true
		}
		return i, false
	}

	i := 0
	for i < n {
		for i < n && content[i] == ' ' {
			i++
		}
		keyStart := i
		for i < n && content[i] != ':' {
			i++
		}
		if i >= n {
			return 0, 0, false
		}
		key := content[keyStart:i]
		i++ // consume ':'

		j, isQuote := quoteAt(i)
		if !isQuote {
			return 0, 0, false
		}
		i = j
		start = i
		for i < n {
			if _, isQuote := quoteAt(i); isQuote {
				break
			}
			i++
		}
		end = i
		j, isQuote = quoteAt(i)
		if !isQuote {
			return 0, 0, false
		}
		i = j
		if key == "json" {
			return start, end, true
		}
	}
	return 0, 0, false
}

// omitemptyOption reports whether the json tag value contains an "omitempty"
// option and, if so, returns its byte offset within val. The first
// comma-separated element is the field name and is skipped, so a field literally
// named "omitempty" (`json:"omitempty"`) is not mistaken for the option.
func omitemptyOption(val string) (off int, ok bool) {
	start := 0
	first := true
	for i := 0; i <= len(val); i++ {
		if i == len(val) || val[i] == ',' {
			if !first && val[start:i] == "omitempty" {
				return start, true
			}
			first = false
			start = i + 1
		}
	}
	return 0, false
}

// isRawMessage reports whether expr denotes encoding/json.RawMessage (or a
// pointer to it). It prefers type information but falls back to a syntactic
// check so it still works under syntax-only load modes.
func isRawMessage(pass *analysis.Pass, expr ast.Expr) bool {
	if pass.TypesInfo != nil {
		if t := pass.TypesInfo.TypeOf(expr); t != nil {
			return isRawMessageType(t)
		}
	}
	return isRawMessageExpr(expr)
}

func isRawMessageType(t types.Type) bool {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == "encoding/json" && obj.Name() == "RawMessage"
}

func isRawMessageExpr(expr ast.Expr) bool {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkgIdent.Name == "json" && sel.Sel.Name == "RawMessage"
}
