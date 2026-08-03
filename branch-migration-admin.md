# Branch migration: admin runbook

This document is for repository administrators completing the `v3` → `dev` + `main` Git Flow migration.

Repository file updates (CI workflows, docs, PR template) are committed on **`v3`**, and `dev` and `main` have been
brought to the same commit. One step remains and it requires an **organization owner**.

## Current status

| Step | Status |
|------|--------|
| `dev` branch created (same tip as `v3`) | Done |
| CI/docs updated to target `dev` | Done (on `v3`) |
| `dev` fast-forwarded to `v3` | Done |
| `main` fast-forwarded to `dev` | Done (was v2.0.0) |
| Branch protection on `dev` / `main` | Done |
| Default branch set to `dev` | **Org owner required** |
| `v3` branch retired | Deferred — `v3` is kept as an alias for now |

`dev`, `main`, and `v3` all point at the same commit.

## Remaining step: set the default branch to `dev`

**Settings → General → Default branch → Switch to `dev`**

Or via CLI:

```bash
gh api repos/aerospike/aerospike-backup-service -X PATCH -f default_branch=dev
```

Repo-admin permission is **not** sufficient — the enterprise restricts this to organization owners. A repo admin gets:

```
422 Validation Failed: You don't have permission to change the default branch.
```

### Why this step matters

The organization ruleset `protect_default_branch_0001` targets `~DEFAULT_BRANCH`, so it follows whichever branch is
default. Switching the default moves that ruleset's protection from `v3` onto `dev` automatically — nothing needs to
be migrated by hand. Until the switch happens, `dev` is covered only by the repo-level branch protection configured
below, and new clones still land on `v3`.

After the switch, retarget any open pull requests from `v3` to `dev`. GitHub only does this automatically when the
base branch is deleted or renamed.

## Branch protection (already applied)

Repo-level protection now configured on both `dev` and `main`:

- Require a pull request with 1 approving review; stale reviews dismissed on push
- Require conversation resolution
- Require branches to be up to date before merging
- Required checks: `build (1.25.x)`, `lint`, `generated-check`, `markdown links`
- No force pushes, no deletions
- `enforce_admins` is off, so repo admins can still push directly in an emergency

## `v3` branch

`v3` is intentionally kept, pointing at the same commit as `dev`, so existing links and clones keep working. Once the
default branch is `dev` and open PRs have been retargeted, it can be deleted with:

```bash
gh api repos/aerospike/aerospike-backup-service/git/refs/heads/v3 -X DELETE
```

Note that `v3` stops being covered by the organization ruleset as soon as it is no longer the default branch.

## Team announcement

Post to the team channel / mailing list:

---

**Branch workflow change — action required**

We are adopting Git Flow:

- **`dev`** — target for all pull requests (replaces `v3`)
- **`main`** — latest release; updated when we cut a release, then tagged (`v3.x.y`)

**Update your local clone:**

```bash
git fetch origin
git branch -m v3 dev            # if you still have a local v3 branch
git branch -u origin/dev dev
git checkout dev
git pull
```

- Bookmarks to `/tree/v3` should use `/tree/dev` or a release tag. `v3` still exists and points at the same commit,
  so nothing breaks immediately.
- `main` now tracks the v3 release line (no longer v2.0.0). v2 maintenance remains on the `v2` branch.
- **Release process:** merge `dev` → `main`, tag on `main`, back-merge `main` → `dev` if needed.

---

## Optional follow-up

- Set Codecov default branch to `main` on [codecov.io](https://codecov.io)
- Confirm Dependabot opens PRs against `dev` (automatic once `dev` is default)

## Release checklist (ongoing)

1. Merge `dev` → `main` (PR)
2. Tag on `main`: `git tag v3.x.y && git push origin v3.x.y`
3. Pre-release workflow runs on tag push (unchanged)
4. Back-merge `main` → `dev` if the release added commits only on `main`
