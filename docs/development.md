# Development guide

This guide covers what you need to build, test, and submit changes to Aerospike Backup Service (ABS). For running a
released build rather than building from source, see the [Run](../README.md#run) section of the README.

## Prerequisites

- Go 1.25 (see `go` directive in [`go.mod`](../go.mod) for the exact minimum version)
- Docker, for building images and running the [Docker Compose](../build/docker-compose/README.md) dev stack
- Node.js (`npx`), used by `make openapi` to convert the generated Swagger spec to OpenAPI 3
- [`golangci-lint`](https://golangci-lint.run/) and [`gci`](https://github.com/daixiang0/gci), used by `make lint`
  and `make format`

## Git submodule

Aerospike cluster configuration validation depends on JSON schemas vendored as a git submodule at
[`modules/schema/schemas`](../modules/schema/schemas) (from
[`aerospike/schemas`](https://github.com/aerospike/schemas)). Clone with submodules, or initialize them afterward:

```bash
git clone --recurse-submodules https://github.com/aerospike/aerospike-backup-service.git

# or, in an existing clone:
make submodules
```

`make build` and CI both run `make submodules` first, so a forgotten submodule checkout mainly bites local
`go build`/`go test` invocations run outside `make`.

## Building and testing

```bash
make build          # release binary under build/target
make build BUILD_MODE=debug  # debug binary with pprof on localhost:6060
make test           # go test -v ./...
```

CI additionally runs tests with the race detector and the `ci` build tag:

```bash
go test -race -tags=ci ./... -coverprofile=coverage.out -covermode=atomic
```

## Generated artifacts

Three sets of files are generated from source and checked into the repository. Each has a `make <x>` target to
regenerate it and a `make <x>-check` target (used in CI) that fails if the committed output is stale:

| What                                             | Regenerate         | Verify              |
|---------------------------------------------------|---------------------|----------------------|
| Mocks for `pkg/service` interfaces (`mockgen.go`)  | `make mocks-generate` | `make mocks-check`  |
| OpenAPI spec (`docs/docs.go`, `docs/openapi.json`, `docs/config.schema.json`) | `make openapi` | `make openapi-check` |
| README and split docs (DTO examples, default config, metrics table) | `make readme` | `make readme-check`  |

`make generated-check` runs all three and is what CI's "Generated files up to date" workflow calls. If you change a
`pkg/dto` struct, an HTTP handler's Swagger annotations, or a Prometheus metric, run the corresponding generator and
commit its output in the same PR — don't hand-edit the generated sections.

## Before opening a pull request

```bash
make pr
```

This runs `go mod tidy`, `mocks-check`, `format`, `lint-fix`, `test`, `openapi`, and `readme` in sequence — the same
checks (plus the race-enabled test run) that CI enforces via the `Build`, `golangci-lint`, and `Generated files up to
date` workflows. Running it locally before pushing avoids a slow feedback loop through CI.

See the [pull request template](../.github/pull_request_template.md) for the full submission checklist.

## Which branch to target

Target pull requests at **`v3`**, the active development branch for the current major version. `main` tracks older
releases and is not where new work lands.

## Reporting issues

Use the [bug report](../.github/ISSUE_TEMPLATE/bug_report.yml) or
[feature request](../.github/ISSUE_TEMPLATE/feature_request.yml) issue forms.
