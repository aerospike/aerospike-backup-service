# Development guide

This guide covers what you need to build, test, and submit changes to Aerospike Backup Service (ABS). For running a
released build rather than building from source, see the [Run](installation.md#run) section of the installation guide.

## Prerequisites

- Go <!-- tag GoVersion -->1.25.13<!-- /tag --> (the `go` directive in [`go.mod`](../go.mod), which is where this number is rendered from)
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

Anything the code already knows is written into a document by `make docs`. Mark an empty region with the
id of what belongs there and the generator fills it in:

```markdown
<!-- tag <id> [args] --><!-- /tag -->
```

The id says what to render, and the ids come from three places, all in one namespace — two
sources claiming the same id is a build failure:

| Source                         | Ids                                                        | Renders                                                         |
|--------------------------------|------------------------------------------------------------|-----------------------------------------------------------------|
| `docs/openapi.json`            | every operation id, e.g. `restoreFull`                     | `` `POST /v1/restore/full` ``, or a linked call-out with `link` |
| `jsonExamples`, `yamlExamples` | e.g. `RestoreFullRequest`                                  | a fenced block built from the DTO structs                       |
| the generator                  | constants like `DefaultConfig`, `Metrics`, `GoVersion` etc | values obtained from the code                                   |

### Validated: everything else

[`internal/doccheck`](../internal/doccheck) puts the hand-written parts through the same parsers the service uses, so
a document that no longer matches the code fails the build rather than a support ticket.

These are ordinary tests and also run under `make test`; the target is for iterating on the docs.

| Check                                   | What it reads                                                                                     | Why it is not generated                                                                                                                                                                                                                                                             |
|-----------------------------------------|---------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Published defaults are the applied ones | `default:` struct tags in `pkg/dto`, against the `pkg/model` code that fills the value in         | Go needs both the tag and the business logic.                                                                                                                                                                                                                                       |
| The OpenAPI contract matches the router | `docs/openapi.json`, against `internal/server.Routes`                                             | Two genuinely independent sources: swag annotations on handlers, and the patterns the mux registers. Everything downstream of the OpenAPI document is generated, so this is the only hop left to check.                                                                             |
| Field names in prose are real           | backticked kebab-case words in those documents, against the property names in `docs/openapi.json` | A sentence can name a field that does not exist — `storage-name` for `source-name`, `parallel-read` for `parallel`. Nothing generates prose, so this can only be checked.                                                                                                           |

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

Releases move through JFrog's promotion stages (`DEV -> TEST -> STAGE -> PREVIEW -> PROD`) before anything is made public. The
GitHub Actions side is split into two workflows: [`pre-release.yml`](../.github/workflows/pre-release.yml) (developer
owned, builds and promotes up to `TEST`) and [`release.yml`](../.github/workflows/release.yml) (run once the release
is fully approved; publishes a GitHub pre-release for final validation, then a PM/EM promotes it to GA manually).

#### Chart versioning

A Helm repository serves whichever chart carries the highest version, so the chart version is a
moving pointer just like the Docker `latest` tag. It must therefore sort in the same order as the
app versions it ships. The rule, in force from `v3.7.0`:

| Release | App version | Chart version |
|---------|-------------|---------------|
| New minor line | `v3.7.0` | `2.1.0` (chart **minor** advances) |
| Hotfix on that line | `v3.7.1` | `2.1.1` (chart **patch** mirrors the app patch) |
| Next minor line | `v3.8.0` | `2.2.0` |

Because a hotfix stays on its own line's chart minor, a fix released for an older line can never
overtake a newer release. This is what went wrong before the rule existed: `v3.5.1` was cut after
`v3.6.2` and took chart `2.0.13`, above `v3.6.2`'s `2.0.12`, so `helm install` with no `--version`
served the older service. [`pre-release.yml`](../.github/workflows/pre-release.yml) now rejects any
chart version that breaks the ordering, reuses a version already published, or whose patch does not
match the app patch.

#### Regular release
1. Create a release branch from `dev` (e.g. `release/3.7.0`).
2. Prepare the release by updating the version files. Pass both versions in one invocation --
   `make release` writes `VERSION` and then stamps the chart from it, so splitting the two leaves
   `Chart.yaml` describing a release that does not exist yet:
   ```bash
   NEXT_VERSION="<version>" NEXT_HELM_CHART_VERSION="<helm-chart-version>" make release
   git add --all
   git commit -m "Release: "$(cat VERSION)""
   ```
3. Open a pull request from your release branch into `main` and merge it.
4. After the PR is merged, tag the release on `main`:
   ```bash
   git checkout main && git pull origin main
   git tag "$(cat VERSION)"
   git push origin main --tags
   ```

#### Hotfix
1. Create a hotfix branch from `main` (e.g. `hotfix/3.6.2`).
2. Prepare the hotfix by updating the version files. Bump the **third digit** of both the version
   (e.g. `3.7.0` -> `3.7.1`) and the Helm chart version (e.g. `2.1.0` -> `2.1.1`), keeping the chart
   on the minor already assigned to that app minor line -- see [Chart versioning](#chart-versioning):
   ```bash
   NEXT_VERSION="<version>" NEXT_HELM_CHART_VERSION="<helm-chart-version>" make release
   git add --all
   git commit -m "Release: "$(cat VERSION)""
   ```
3. **Do not merge** the hotfix branch into `main`. Tag and push the hotfix directly from the branch:
   ```bash
   git tag "$(cat VERSION)"
   git push origin hotfix/3.6.2 --tags
   ```

#### Promotion and publication
The following steps apply to both regular releases and hotfixes:

1. Tagging the release commit triggers `pre-release.yml`, which:
   1. Builds the DEB/RPM packages, Helm chart, and Docker image.
   2. Signs the packages and Helm chart, and deploys everything to JFrog `DEV`.
   3. Creates a unified release bundle and automatically promotes it from `DEV` to `TEST`.
6. QE/developers pull the artifacts from JFrog `TEST` and validate them. Once they pass, the release bundle is
   promoted from `TEST` to `STAGE`, either by dispatching
   [`promote-to-preview.yml`](https://github.com/aerospike/aerospike-backup-service/actions/workflows/promote-to-preview.yml) with `environment: STAGE` or manually via the
   [JFrog UI](https://aerospike.jfrog.io/ui/artifactory/release-lifecycle/aerospike-backup-service?repoKey=database-release-bundles-v2).
7. A PM or EM reviews the release and promotes the release bundle from `STAGE` to `PREVIEW`, either by dispatching
   [`promote-to-preview.yml`](https://github.com/aerospike/aerospike-backup-service/actions/workflows/promote-to-preview.yml) with `environment: PREVIEW` or manually via the same
   [JFrog UI](https://aerospike.jfrog.io/ui/artifactory/release-lifecycle/aerospike-backup-service?repoKey=database-release-bundles-v2)
   link.
8. A PM or EM promotes the release bundle from `PREVIEW` to `PROD`, either by dispatching
   [`promote-to-prod.yml`](https://github.com/aerospike/aerospike-backup-service/actions/workflows/promote-to-prod.yml) or manually via the same JFrog UI link. This is
   the gate that makes a release public.
9. Once the bundle is on `PROD`:
   - Docker Hub and every other public registry are published automatically and externally by the central
     `artifact-publisher` repository, off JFrog's `release_bundle_v2_promotion_completed` webhook — nothing
     to trigger here, and nothing in this repository ever pushes to an external registry
     ([strategy](https://aerospike.atlassian.net/wiki/spaces/DevOps/pages/4648566799)).
   - A dev or PM/EM manually runs [`release.yml`](https://github.com/aerospike/aerospike-backup-service/actions/workflows/release.yml)
     (`workflow_dispatch`, with the release version as input). It verifies the bundle was actually promoted to
     `PROD`, then downloads the already-signed artifacts straight from JFrog's `PROD`-public repos and publishes
     them as a new, immutable GitHub **pre-release** — nothing is rebuilt, re-signed, or re-checksummed at
     this point.
10. When ready to announce GA, a PM/EM edits that GitHub Release and clears **Set as a pre-release** only.
    The GitHub **Set as the latest release** checkbox is unrelated to any registry tag and can be left
    unchecked. Until the pre-release flag is cleared, the release does not appear as GA on GitHub.
11. Post-release actions (after step 10):
   1. **Snyk**:
      - Add the new version to the `aerospike-applications` Snyk org (monitor the Docker image).
      - Remove the oldest maintenance version from the same org if no longer supported.
   2. **Slack**:
      - Post the release announcement to the internal **`#releases`** channel.
      - Use the link to the [prettified release notes](https://aerospike.com/docs/database/tools/backup-and-restore/backup-service/release/) if available; otherwise, use the GitHub Release link.
      - **Important**: Remove link previews before sending to keep the channel clean (hover over the preview and click the **'x'** in the top-right corner). See [this guide](https://aerospike.atlassian.net/wiki/spaces/RE/pages/2540339350/Message+Slack+releases+Internal+Channel) for more info.
   3. **Email**: Send the release announcement email to the appropriate internal distribution lists. See [this guide](https://aerospike.atlassian.net/wiki/spaces/RE/pages/2543124552/Send+email+of+the+Release+Notes+to+the+releases+aerospike.com+distribution+list) for more info.
12. If the release added commits that exist only on `main` (for example a hotfix), back-merge `main` into `dev`.

## Reporting issues

Use the [bug report](../.github/ISSUE_TEMPLATE/bug_report.yml) or
[feature request](../.github/ISSUE_TEMPLATE/feature_request.yml) issue forms.
