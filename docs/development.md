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

## Branching model

The repository follows Git Flow:

| Branch      | Role                                                                            |
|-------------|---------------------------------------------------------------------------------|
| `dev`       | Integration branch and the default branch. All pull requests target it.          |
| `main`      | Latest release. Updated only by a release PR from `dev`, then tagged `v3.x.y`.    |
| `v2`        | Maintenance for the 2.x line.                                                     |
| `feature/*`, `bugfix/*` | Short-lived branches off `dev`, merged back via pull request.         |
| `hotfix/*`  | Branched off `main` for urgent fixes; merged into both `main` and `dev`.          |

Target your pull requests at **`dev`**. Do not open feature pull requests against `main` — it only moves forward
through releases and hotfixes.

### Cutting a release

Releases move through JFrog's promotion stages (`DEV -> TEST -> STAGE -> PROD`) before anything is made public. The
GitHub Actions side is split into two workflows: [`pre-release.yml`](../.github/workflows/pre-release.yml) (developer
owned, builds and promotes up to `TEST`) and [`release.yml`](../.github/workflows/release.yml) (run once the release
is fully approved, publishes it).

1. Open a pull request from `dev` into `main` and merge it.
2. Tag the release commit on `main`: `git tag v3.x.y && git push origin v3.x.y`. This triggers `pre-release.yml`,
   which:
   1. Builds the DEB/RPM packages, Helm chart, and Docker image.
   2. Signs the packages and Helm chart, and deploys everything to JFrog `DEV`.
   3. Creates a unified release bundle and automatically promotes it from `DEV` to `TEST`.
3. QE/developers pull the artifacts from JFrog `TEST` and validate them. Once they pass, the release bundle is
   promoted from `TEST` to `STAGE` (manually, via the JFrog UI — not automated).
4. A PM or EM reviews the release and promotes the release bundle from `STAGE` to `PROD` via the JFrog UI. This is
   the gate that makes a release public; automation intentionally stops before this step.
5. Once the bundle is on `PROD`:
   - Docker Hub mirroring happens automatically and externally (JFrog's existing promotion webhook feeds
     `artifact-publisher`) — nothing to trigger here.
   - Someone with repo access (not necessarily the same PM/EM from step 4) manually runs `release.yml`
     (`workflow_dispatch`, with the release version as input). It verifies the bundle was actually promoted to
     `PROD`, then downloads the already-signed artifacts straight from JFrog's `PROD`-public repos and publishes
     them as a new, immutable GitHub Release — nothing is rebuilt, re-signed, or re-checksummed at this point.
6. If the release added commits that exist only on `main` (for example a hotfix), back-merge `main` into `dev`.

## Reporting issues

Use the [bug report](../.github/ISSUE_TEMPLATE/bug_report.yml) or
[feature request](../.github/ISSUE_TEMPLATE/feature_request.yml) issue forms.
