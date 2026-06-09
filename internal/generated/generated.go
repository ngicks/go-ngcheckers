// Package generated identifies machine-generated Go source files so checkers
// can skip them.
//
// A checker should not flag code that a human cannot edit by hand. Generated
// files carry the conventional marker comment described at
// https://go.dev/s/generatedcode ("// Code generated ... DO NOT EDIT.") before
// their package clause; [go/ast.IsGenerated] detects it. This package collects
// the generated files of a package once and answers, per AST node position,
// whether that node lives in one — so a checker built on
// golang.org/x/tools/go/ast/inspector can skip generated nodes without
// re-scanning comments for every node.
package generated

import (
	"go/ast"
	"go/token"
)

// Files records which files of a single package go/ast considers generated.
// The zero value is not usable; construct one with [Collect].
type Files struct {
	fset *token.FileSet
	gen  map[*token.File]struct{}
}

// Collect scans files and records those that [ast.IsGenerated] reports as
// generated. fset must be the FileSet the files were parsed with; the files
// must have been parsed with parser.ParseComments (as go/analysis always does),
// otherwise the generated marker is invisible and nothing is recorded.
func Collect(fset *token.FileSet, files []*ast.File) *Files {
	gen := make(map[*token.File]struct{})
	for _, f := range files {
		if !ast.IsGenerated(f) {
			continue
		}
		// FileStart is guaranteed to fall within the file, so it resolves to
		// the same *token.File that any node position in the file resolves to.
		if tf := fset.File(f.FileStart); tf != nil {
			gen[tf] = struct{}{}
		}
	}
	return &Files{fset: fset, gen: gen}
}

// Contains reports whether pos lies within a generated file. A nil receiver,
// an invalid pos, or a pos from another FileSet reports false.
func (f *Files) Contains(pos token.Pos) bool {
	if f == nil {
		return false
	}
	_, ok := f.gen[f.fset.File(pos)]
	return ok
}
