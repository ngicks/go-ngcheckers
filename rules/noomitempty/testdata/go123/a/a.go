// Package a uses "omitempty" on a module targeting Go 1.23, where "omitzero"
// does not exist. The analyzer must be a no-op here, so there are no "want"
// comments: any diagnostic would fail the test.
package a

type S struct {
	Name string `json:"name,omitempty"`
	Opt  int    `json:"opt,omitempty,string"`
	Bare string `json:",omitempty"`
}
