// Package directive parses ngcheckers directive comments and reports which
// rules (checkers) they name.
//
// The only directive today is //ngignore, which suppresses one or more checkers
// on the annotated node:
//
//	//ngignore:<rule>[,<rule>...] [free-form reason]
//
// Unlike golangci-lint's //nolint — which golangci-lint applies after the fact
// and which go vet and the standalone driver never see — these directives are
// honored by the checkers themselves, so suppression works in every run mode,
// including the editor/hook integrations that drive this tool.
package directive

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// IgnorePrefix is the comment prefix marking an //ngignore directive. Per Go
// directive convention there is no space between "//" and the directive name.
const IgnorePrefix = "//ngignore:"

// Ignore is a parsed //ngignore directive: the rule (checker) names it silences
// and the free-form reason that follows them.
type Ignore struct {
	Rules  []string
	Reason string
}

// ParseIgnore parses comment as an //ngignore directive, returning ok=false
// when comment is not one. The rule list is comma-separated with no internal
// spaces (as with golangci-lint's //nolint); the first run of whitespace after
// the list ends it, and whatever follows becomes Reason. Whitespace right after
// the colon and around each rule name is trimmed, and empty names are dropped.
func ParseIgnore(comment string) (ig Ignore, ok bool) {
	rest, ok := strings.CutPrefix(comment, IgnorePrefix)
	if !ok {
		return Ignore{}, false
	}
	rest = strings.TrimLeft(rest, " \t")
	list := rest
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		list = rest[:i]
		ig.Reason = strings.TrimSpace(rest[i+1:])
	}
	for n := range strings.SplitSeq(list, ",") {
		if n = strings.TrimSpace(n); n != "" {
			ig.Rules = append(ig.Rules, n)
		}
	}
	return ig, true
}

// Names reports whether the directive lists rule among its rule names.
func (ig Ignore) Names(rule string) bool {
	return slices.Contains(ig.Rules, rule)
}

// SuppressesLine reports whether an //ngignore directive naming rule appears
// on the same line as pos or on the line directly above it, anywhere in file's
// comments. It serves nodes the parser attaches no comments to (expressions,
// statements), where [Suppresses] cannot be used: the directive is written
// either trailing the offending line or on its own line right above it. pos
// must belong to file and fset must be the FileSet file was parsed with;
// otherwise the result is false.
func SuppressesLine(fset *token.FileSet, file *ast.File, rule string, pos token.Pos) bool {
	tf := fset.File(pos)
	if tf == nil || file == nil {
		return false
	}
	line := tf.Line(pos)
	for _, g := range file.Comments {
		for _, c := range g.List {
			// Comments of file resolve within the same *token.File as pos.
			cl := tf.Line(c.Pos())
			if cl != line && cl != line-1 {
				continue
			}
			if ig, ok := ParseIgnore(c.Text); ok && ig.Names(rule) {
				return true
			}
		}
	}
	return false
}

// Suppresses reports whether any //ngignore directive among the given comment
// groups names rule. Pass a node's lead and trailing comment groups — for a
// struct field, field.Doc and field.Comment. Nil groups are skipped.
func Suppresses(rule string, groups ...*ast.CommentGroup) bool {
	for _, g := range groups {
		if g == nil {
			continue
		}
		for _, c := range g.List {
			if ig, ok := ParseIgnore(c.Text); ok && ig.Names(rule) {
				return true
			}
		}
	}
	return false
}
