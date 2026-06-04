package noomitempty_test

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ngicks/go-ngcheckers/rules/noomitempty"
)

// TestAnalyzerGo124 runs against a module targeting Go 1.24, where "omitzero"
// is available. It checks both the reported diagnostics ("// want" comments)
// and the suggested fixes (the *.golden files).
func TestAnalyzerGo124(t *testing.T) {
	dir := filepath.Join(analysistest.TestData(), "go124")
	analysistest.RunWithSuggestedFixes(t, dir, noomitempty.Analyzer, "example.com/go124/a")
}

// TestAnalyzerGo123 runs against a module targeting Go 1.23, where "omitzero"
// does not exist. The analyzer must be a no-op: the testdata has no "// want"
// comments, so any diagnostic fails the test.
func TestAnalyzerGo123(t *testing.T) {
	dir := filepath.Join(analysistest.TestData(), "go123")
	analysistest.Run(t, dir, noomitempty.Analyzer, "example.com/go123/a")
}
