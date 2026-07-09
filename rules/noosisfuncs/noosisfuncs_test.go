package noosisfuncs_test

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ngicks/go-ngcheckers/rules/noosisfuncs"
)

// TestAnalyzerGo124 runs against a module targeting Go 1.24, where every
// errors.Is replacement is available. It checks both the reported diagnostics
// ("// want" comments) and the suggested fixes (the *.golden files).
func TestAnalyzerGo124(t *testing.T) {
	dir := filepath.Join(analysistest.TestData(), "go124")
	analysistest.RunWithSuggestedFixes(t, dir, noosisfuncs.Analyzer, "example.com/go124/a")
}

// TestAnalyzerGo115 runs against a module targeting Go 1.15:
// os.ErrDeadlineExceeded exists, so os.IsTimeout is reported, but package
// io/fs does not exist until Go 1.16, so the other predicates are not.
func TestAnalyzerGo115(t *testing.T) {
	dir := filepath.Join(analysistest.TestData(), "go115")
	analysistest.Run(t, dir, noosisfuncs.Analyzer, "example.com/go115/a")
}

// TestAnalyzerGo112 runs against a module targeting Go 1.12, below every
// predicate's replacement. The analyzer must be a no-op: the testdata has no
// "// want" comments, so any diagnostic fails the test.
func TestAnalyzerGo112(t *testing.T) {
	dir := filepath.Join(analysistest.TestData(), "go112")
	analysistest.Run(t, dir, noosisfuncs.Analyzer, "example.com/go112/a")
}
