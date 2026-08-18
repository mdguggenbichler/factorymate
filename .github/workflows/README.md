# GitHub Actions workflows

Entry-point workflows own triggers; reusable workflows (prefixed `_`) are called via `workflow_call` only.

> **Note:** GitHub Actions requires reusable workflows at the top level of `.github/workflows/` — subdirectories are not supported.

## Workflow map

| Entry point | Trigger | Calls |
|-------------|---------|-------|
| `pr.yml` | `pull_request` | `_ci.yml` |
| `dev.yml` | `push` → `dev` | `_ci.yml` → `_container-image.yml` |
| `main.yml` | `push` → `main` | `_release-draft.yml` |
| `release.yml` | `release` → `published` (`v*`) | `_container-image.yml`, MkDocs → GitHub Pages |

## Trigger rules

- **`on.push`**: only `main.yml` (`main`) and `dev.yml` (`dev`)
- **`on.pull_request`**: only `pr.yml`
- **Reusable** (`_*.yml`): `workflow_call` only — no direct push/PR triggers

## CI (`_ci.yml`)

Runs backend (`go build`, `go vet`, `go test`), frontend (`npm ci`, `lint`, `build`), and a Docker smoke build (`push: false`).

## Container images (`_container-image.yml`)

Single image: `ghcr.io/<owner>/factorymate` (repository name lowercased).

| Trigger | Tags pushed | Docs |
|---------|-------------|------|
| `push` → `dev` | `nightly`, `{sha7}` | — |
| `release` published (`v1.2.3`) | `v1.2.3`, `latest` | Deploy to GitHub Pages |

## Releases (`_release-draft.yml`)

On `push` → `main`, compares root [`VERSION`](../../VERSION) at HEAD against the latest `v*` tag. When semver increased and `v{VERSION}` does not exist yet, creates the git tag and a **draft** GitHub release. Publishing the release triggers `release.yml` to push stable images.
