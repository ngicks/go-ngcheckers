package directive_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/ngicks/go-ngcheckers/internal/directive"
)

func TestParseIgnore(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		wantOK     bool
		wantRules  []string
		wantReason string
	}{
		{
			name:    "not a directive",
			comment: "// just a plain comment",
			wantOK:  false,
		},
		{
			name:    "nolint is not ngignore",
			comment: "//nolint:noomitempty",
			wantOK:  false,
		},
		{
			name:    "space after slashes is not the strict prefix",
			comment: "// ngignore:noomitempty",
			wantOK:  false,
		},
		{
			name:      "single rule, no reason",
			comment:   "//ngignore:noomitempty",
			wantOK:    true,
			wantRules: []string{"noomitempty"},
		},
		{
			name:       "single rule with reason",
			comment:    "//ngignore:noomitempty zero value is meaningful",
			wantOK:     true,
			wantRules:  []string{"noomitempty"},
			wantReason: "zero value is meaningful",
		},
		{
			name:       "comma-separated rules with reason",
			comment:    "//ngignore:foo,noomitempty,bar intentional",
			wantOK:     true,
			wantRules:  []string{"foo", "noomitempty", "bar"},
			wantReason: "intentional",
		},
		{
			name:       "whitespace right after colon is tolerated",
			comment:    "//ngignore:  noomitempty spaced out",
			wantOK:     true,
			wantRules:  []string{"noomitempty"},
			wantReason: "spaced out",
		},
		{
			name:    "empty rule list",
			comment: "//ngignore:",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ig, ok := directive.ParseIgnore(tt.comment)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !slices.Equal(ig.Rules, tt.wantRules) {
				t.Errorf("Rules = %q, want %q", ig.Rules, tt.wantRules)
			}
			if ig.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", ig.Reason, tt.wantReason)
			}
		})
	}
}

func TestIgnoreNames(t *testing.T) {
	ig, _ := directive.ParseIgnore("//ngignore:foo,noomitempty done")
	if !ig.Names("noomitempty") {
		t.Error("Names(noomitempty) = false, want true")
	}
	if ig.Names("missing") {
		t.Error("Names(missing) = true, want false")
	}
}

func TestSuppresses(t *testing.T) {
	group := func(texts ...string) *ast.CommentGroup {
		cg := &ast.CommentGroup{}
		for _, s := range texts {
			cg.List = append(cg.List, &ast.Comment{Text: s})
		}
		return cg
	}

	lead := group("//ngignore:noomitempty deliberate")
	trailing := group("//ngignore:othercheck,noomitempty x")
	unrelated := group("// regular doc", "//ngignore:othercheck only")

	tests := []struct {
		name   string
		rule   string
		groups []*ast.CommentGroup
		want   bool
	}{
		{
			name:   "lead group names rule",
			rule:   "noomitempty",
			groups: []*ast.CommentGroup{lead, nil},
			want:   true,
		},
		{
			name:   "trailing group names rule",
			rule:   "noomitempty",
			groups: []*ast.CommentGroup{nil, trailing},
			want:   true,
		},
		{
			name:   "only other rule named",
			rule:   "noomitempty",
			groups: []*ast.CommentGroup{unrelated},
			want:   false,
		},
		{name: "no groups", rule: "noomitempty", want: false},
		{
			name:   "all nil groups",
			rule:   "noomitempty",
			groups: []*ast.CommentGroup{nil, nil},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := directive.Suppresses(tt.rule, tt.groups...); got != tt.want {
				t.Errorf("Suppresses(%q, ...) = %v, want %v", tt.rule, got, tt.want)
			}
		})
	}
}

func TestSuppressesLine(t *testing.T) {
	const src = `package p

import "os"

func f(err error) {
	_ = os.IsNotExist(err) //ngignore:noosisfuncs trailing directive

	//ngignore:noosisfuncs lead directive
	_ = os.IsNotExist(err)

	//ngignore:othercheck names another rule
	_ = os.IsNotExist(err)

	//ngignore:noosisfuncs two lines above, too far
	// filler
	_ = os.IsNotExist(err)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	// Collect the os.IsNotExist call positions in source order.
	var calls []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call.Pos())
		}
		return true
	})
	if len(calls) != 4 {
		t.Fatalf("found %d calls, want 4", len(calls))
	}

	wants := []struct {
		name string
		want bool
	}{
		{name: "trailing directive on the same line", want: true},
		{name: "directive on the line directly above", want: true},
		{name: "directive naming another rule", want: false},
		{name: "directive two lines above", want: false},
	}
	for i, tt := range wants {
		t.Run(tt.name, func(t *testing.T) {
			if got := directive.SuppressesLine(
				fset,
				file,
				"noosisfuncs",
				calls[i],
			); got != tt.want {
				t.Errorf("SuppressesLine(call %d) = %v, want %v", i, got, tt.want)
			}
		})
	}

	if directive.SuppressesLine(token.NewFileSet(), file, "noosisfuncs", calls[0]) {
		t.Error("SuppressesLine with a foreign FileSet = true, want false")
	}
}
