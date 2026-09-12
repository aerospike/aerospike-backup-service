# Development guide

This guide covers what you need to build, test, and submit changes to Aerospike Backup Service (ABS). For running a
released build rather than building from source, see the [Run](../README.md#run) section of the README.

## Prerequisites

- Go 1.25 (see `go` directive in [`go.mod`](../go.mod) for the exact minimum version)
- Docker, for building images and running the [Docker Compose](../build/docker-compose/README.md) dev stack
- Node.js (`npx`), used by `make docs` to convert the generated Swagger spec to OpenAPI 3
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

`make deadcode` reports functions unreachable from `cmd/backup`. `pkg/validation` is ignored: it is a standalone check API, not called from the service binary.

## Keeping the documentation true

Prose goes stale in a way generated documentation cannot, and the two problems have different answers. The rule this
repository follows is **generate the mechanical facts, validate the authored ones**: a fact the code already owns —
a path, a method, a schema — should be rendered rather than retyped, while a thing a person deliberately wrote —
an example, an explanation — can only be checked.

### Generated: tags

Anything the code already knows is written into a document by `make docs`, not typed. Mark an empty region with the
id of what belongs there and the generator fills it in:

```markdown
<!-- tag <id> [args] --><!-- /tag -->
```

One form for everything. The id says what to render, and the ids come from three places, all in one namespace — two
sources claiming the same id is a build failure:

| Source | Ids | Renders |
|---|---|---|
| `docs/openapi.json` | every operation id, e.g. `restoreFull` | `` `POST /v1/restore/full` ``, or a linked call-out with `link` |
| `jsonExamples`, `yamlExamples` | e.g. `RestoreFullRequest` | a fenced block built from the DTO structs |
| the generator | `DefaultConfig`, `Metrics`, `TLSReloadInterval`, `RBACMatrix` | the packaged config, the metrics table, the reload interval |

So an endpoint reads as either of:

```markdown
A client calls <!-- tag restoreFull --><!-- /tag --> to begin.
<!-- tag getFullBackupsForRoutine link ?from=<from>&to=<to> --><!-- /tag -->
```

A query string stays with the author, because which parameters an example demonstrates is a teaching decision rather
than a fact about the API. The same id may appear in as many documents as you like, in either form.

Three mistakes stop the build rather than silently generating nothing: an unknown id, a missing `<!-- /tag -->`, and
an unknown argument. The closing marker is what lets one rule cover both a call-out on its own line and a code span
mid-sentence — it says exactly how far the generated text reaches, so a region never swallows the prose after it and
two tags in one sentence stay separate. The literal `tag` keyword is what distinguishes these from markers left by
other tools, such as the `<!-- toc -->` the table-of-contents generator writes. HTML comments are invisible in
rendered Markdown.

### Validated: everything else

[`internal/doccheck`](../internal/doccheck) puts the hand-written parts through the same parsers the service uses, so
a document that no longer matches the code fails the build rather than a support ticket.

```bash
make doc-check
```

These are ordinary tests and also run under `make test`; the target is for iterating on the docs.

| Check | What it reads | Why it is not generated |
|---|---|---|
| Published defaults are the applied ones | `default:` struct tags in `pkg/dto`, against the `pkg/model` code that fills the value in | Go needs both the tag and the business logic. A mismatch is a question for a person — `maxage` publishes 7 days while nothing applies it, and only a human knows whether the tag or the code was the mistake. A generator would answer "the code" and the intent would vanish. |
| The OpenAPI contract matches the router | `docs/openapi.json`, against `internal/server.Routes` | Two genuinely independent sources: swag annotations on handlers, and the patterns the mux registers. Everything downstream of the OpenAPI document is generated, so this is the only hop left to check. |
| Field names in prose are real | backticked kebab-case words in those documents, against the property names in `docs/config.schema.json` and `docs/openapi.json` | A sentence can name a field that does not exist — `storage-name` for `source-name`, `parallel-read` for `parallel`. Nothing generates prose, so this can only be checked. |
| Configuration examples decode | Hand-written YAML blocks in the documents listed in `docFiles` | You want to author an example in YAML, not in Go. The check only confirms it still parses. Shipped configuration files are covered elsewhere: `make docs` decodes and validates the packaged one, and the `validate-config-files` workflow checks every `aerospike-backup-service.yml` against the JSON schema. |

Routes are declared once, in `internal/server.Routes`, and registered from that list — which is what lets the route
check read the endpoint set without starting a server. A snippet that `build/docs` renders is skipped by the snippet
check: `make docs-check` already verifies it byte-for-byte.

The documents under check are listed in `docFiles` rather than discovered by scanning `docs/`, so a local run matches
a run in CI even when the working tree holds untracked drafts.

### Coverage

Run the same filtered coverage total that CI and Codecov use:

```bash
make test-cover          # prints filtered total (last line)
make test-cover-html     # also writes coverage.html from the filtered profile
```

[`.covignore`](../.covignore) removes lines from the uploaded profile before the total is computed. It excludes
generated mocks, entrypoints, and packages that are thin wrappers or hard to unit-test in isolation:

| Excluded path | Reason |
|---------------|--------|
| `/cmd/` | CLI entrypoint |
| `/docs/`, `/modules/` | Non-Go assets |
| `/build/` | Build tooling and packaging, not the service |
| `/pkg/model/` | Data structs with no logic |
| `*mockgen.go` | Generated mocks |

`internal/` (HTTP handlers, server wiring) **is** measured. CI fails if filtered coverage drops below the threshold
configured in [`.github/workflows/build.yml`](../.github/workflows/build.yml) — currently **80%**. That threshold
ratchets up as test coverage improves across follow-up PRs.

## Generated artifacts

Two sets of files are generated from source and checked into the repository. Each has a `make <x>` target to
regenerate it and a `make <x>-check` target (used in CI) that fails if the committed output is stale:

| What                                             | Regenerate         | Verify              |
|---------------------------------------------------|---------------------|----------------------|
| Mocks for `pkg/service` interfaces (`mockgen.go`)  | `make mocks-generate` | `make mocks-check`  |
| OpenAPI spec, config schema, README, examples, and DTO markdown | `make docs` | `make docs-check` |

`make generated-check` runs both and is what CI's "Generated files up to date" workflow calls. If you change a
`pkg/dto` struct, an HTTP handler's Swagger annotations, or a Prometheus metric, run `make docs` and
commit its output in the same PR — don't hand-edit the generated sections.

## Before opening a pull request

```bash
make pr
```

This runs `go mod tidy`, `mocks-check`, `format`, `lint-fix`, `test`, and `docs` in sequence — the same
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

1. Open a pull request from `dev` into `main` and merge it.
2. Tag the release commit on `main`: `git tag v3.x.y && git push origin v3.x.y`. The pre-release workflow runs on the
   tag push.
3. If the release added commits that exist only on `main` (for example a hotfix), back-merge `main` into `dev`.

## Reporting issues

Use the [bug report](../.github/ISSUE_TEMPLATE/bug_report.yml) or
[feature request](../.github/ISSUE_TEMPLATE/feature_request.yml) issue forms.
