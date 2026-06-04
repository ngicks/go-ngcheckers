# go-ngcheckers

A bundle of [`go/analysis`][analysis] checkers by ngicks, exposed through a
single driver, `ngcheckers`, that runs both standalone and as a `go vet` tool.

```
.
├── cmd
│   └── ngcheckers      # single entry point (multichecker driver)
└── rules
    └── noomitempty     # one checker per package
```

## Checkers

| Name          | Description |
|---------------|-------------|
| `noomitempty` | Forbids the `omitempty` option in `json` struct tags and suggests `omitzero` (Go 1.24+). No-op for modules targeting Go 1.23 or earlier. `json.RawMessage` fields are exempt. Ships an autofix. |

Run `ngcheckers help <name>` for a checker's full documentation and flags.

## Install

```sh
go install github.com/ngicks/go-ngcheckers/cmd/ngcheckers@latest
# or, from a clone:
go build -o ./bin/ngcheckers ./cmd/ngcheckers
```

## Usage

### Standalone

```sh
ngcheckers ./...                  # run every checker (no flag = all)
ngcheckers -fix ./...             # run every checker and apply suggested fixes
ngcheckers help                   # general help
ngcheckers help noomitempty       # one checker's docs + flags
```

Selecting which checkers run (multichecker convention):

```sh
ngcheckers -noomitempty ./...        # run ONLY noomitempty
ngcheckers -noomitempty=false ./...  # run everything EXCEPT noomitempty
```

- No `-NAME` flag → all checkers run.
- Any `-NAME` set true → run only those (allow-list).
- Otherwise any `-NAME=false` → run all but those (deny-list).
- Per-checker flags are namespaced as `-NAME.flag`.

### As a `go vet` tool

`ngcheckers` speaks the `go vet -vettool` protocol, so it plugs into the Go
build cache (incremental, per-package, parallel):

```sh
go vet -vettool=$(which ngcheckers) ./...                 # all checkers
go vet -vettool=$(which ngcheckers) -noomitempty ./...    # only noomitempty
```

(Use an absolute path to the binary; `$(which ngcheckers)` resolves it from
`$GOPATH/bin` after `go install`.)

## Adding a checker

1. Add a package under `rules/<name>/` that exports an
   `*analysis.Analyzer` named `Analyzer`.
2. Register it in [`cmd/ngcheckers/main.go`](./cmd/ngcheckers/main.go):

   ```go
   multichecker.Main(
       noomitempty.Analyzer,
       yourrule.Analyzer, // add here
   )
   ```

It is then available in every mode above, with its own `-yourrule` selection
flag and `help yourrule` documentation.

[analysis]: https://pkg.go.dev/golang.org/x/tools/go/analysis
