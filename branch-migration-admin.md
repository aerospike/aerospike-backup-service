# Branch migration: admin runbook

This document is for repository administrators completing the `v3` → `dev` + `main` Git Flow migration.

Repository file updates (CI workflows, docs, PR template) are committed on **`v3`**. Remaining steps require GitHub
organization admin access.

## Current status

| Step | Status |
|------|--------|
| `dev` branch created (same tip as `v3`) | Done |
| CI/docs updated to target `dev` | Done (on `v3`) |
| `dev` fast-forwarded to `v3` | **Admin required** |
| Default branch set to `dev` | **Admin required** |
| `main` fast-forwarded to `dev` | **Admin required** (protected) |
| `v3` branch retired | **Admin required** |
| Branch protection on `dev` / `main` | **Admin required** |

## 1. Fast-forward `dev` to `v3`

`dev` was branched from `v3` before the CI/docs updates landed, so it is behind by those commits.

```bash
git fetch origin
git push origin origin/v3:refs/heads/dev
```

## 2. Set default branch to `dev`

**Settings → General → Default branch → Switch to `dev`**

Or via CLI (org admin):

```bash
gh api repos/aerospike/aerospike-backup-service -X PATCH -f default_branch=dev
```

Retarget any open pull requests from `v3` to `dev` (GitHub does this automatically only when the base branch is
deleted or renamed).

## 3. Fast-forward `main` to match `dev`

`main` is currently protected and still points at v2.0.0. `main` is an ancestor of `dev`, so this is a fast-forward.

### Option A: PR merge (recommended)

1. Open a PR: base `main`, compare `dev`
2. Title: `chore: align main with dev for Git Flow release branch`
3. Merge (admin may need to bypass required reviews; the histories do not diverge)

### Option B: Direct push (requires temporarily relaxing `main` protection)

```bash
git fetch origin
git push origin origin/dev:refs/heads/main
```

## 4. Retire the `v3` branch

`dev` already exists, so the rename path (`branches/v3/rename`) is unavailable — delete `v3` once `dev` is the default
branch and open PRs have been retargeted:

```bash
gh api repos/aerospike/aerospike-backup-service/git/refs/heads/v3 -X DELETE
```

`v3` is covered by organization rulesets, so this may require org admin.

## 5. Branch protection

Configure in **Settings → Branches** (or organization rulesets):

| Branch | Rules |
|--------|-------|
| `dev` | Require PR before merge; required checks: Build, golangci-lint, Generated files up to date, Link check; no direct pushes |
| `main` | Require PR before merge; restrict merges to release PRs from `dev` (or release-manager bypass); same required checks |

Migrate any existing `v3` ruleset entries to `dev`.

## 6. Team announcement

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

- Bookmarks to `/tree/v3` should use `/tree/dev` or a release tag.
- `main` now tracks the v3 release line (no longer v2.0.0). v2 maintenance remains on the `v2` branch.
- **Release process:** merge `dev` → `main`, tag on `main`, back-merge `main` → `dev` if needed.

---

## 7. Optional follow-up

- Set Codecov default branch to `main` on [codecov.io](https://codecov.io)
- Confirm Dependabot opens PRs against `dev` (automatic once `dev` is default)

## Release checklist (ongoing)

1. Merge `dev` → `main` (PR)
2. Tag on `main`: `git tag v3.x.y && git push origin v3.x.y`
3. Pre-release workflow runs on tag push (unchanged)
4. Back-merge `main` → `dev` if the release added commits only on `main`
