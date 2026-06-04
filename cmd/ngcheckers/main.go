// Command ngcheckers bundles ngicks's go/analysis checkers into a single
// driver built on golang.org/x/tools/go/analysis/multichecker.
//
// It runs two ways:
//
//	ngcheckers ./...                       # standalone
//	go vet -vettool=$(which ngcheckers) ./...   # as a go vet tool
//
// Checker selection follows multichecker's convention: with no -NAME flag every
// checker runs. Pass -NAME to run only that checker, or -NAME=false to run all
// checkers except it. Per-checker flags are namespaced as -NAME.flag, and
// `ngcheckers help [NAME]` prints documentation.
//
// Register additional checkers by adding their *analysis.Analyzer to the call
// below.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/ngicks/go-ngcheckers/rules/noomitempty"
)

func main() {
	multichecker.Main(
		noomitempty.Analyzer,
	)
}
