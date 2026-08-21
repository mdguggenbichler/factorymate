# GitHub Actions workflows

Entry-point workflows own triggers; reusable workflows (prefixed `_`) are called via `workflow_call` only.

> **Note:** GitHub Actions requires reusable workflows at the top level of `.github/workflows/` — subdirectories are not supported.

## Workflow map

| Entry point | Trigger | Calls |
|-------------|---------|-------|
| `pr.yml` | `pull_request` | `_ci.yml` (`include_docker: true`) |
| `dev.yml` | `push` → `dev` | `_ci.yml` (`include_docker: false`) → `_container-image.yml` |
| `main.yml` | `push` → `main` | `_release-draft.yml` |
| `release.yml` | `release` → `published` (`v*`) | `_container-image.yml`, MkDocs → GitHub Pages |

## Trigger rules

- **`on.push`**: only `main.yml` (`main`) and `dev.yml` (`dev`)
- **`on.pull_request`**: only `pr.yml`
- **Reusable** (`_*.yml`): `workflow_call` only — no direct push/PR triggers

## CI (`_ci.yml`)

Reusable CI with optional Docker smoke build via `include_docker` (default `true`).

| Job | What |
|-----|------|
| Backend | `go build`, `go vet`, `go test` |
| Frontend | `npm ci`, `lint`, `build` |
| Docker smoke | Full image build, `push: false` — **only when `include_docker: true`** |

**Who calls it:**

| Caller | `include_docker` | Why |
|--------|------------------|-----|
| `pr.yml` | `true` (default) | PR merge gate — catches Docker-only failures (layout, COPY, `.dockerignore`) that standalone frontend build misses |
| `dev.yml` | `false` | `container-image` builds the same Dockerfile once and pushes; avoids duplicate build per push |

Both Docker jobs use GHA BuildKit layer cache (`cache-from` / `cache-to`); the smoke job does **not** export a finished image for the push step.

## Container images (`_container-image.yml`)

Single image: `ghcr.io/<owner>/factorymate` (repository name lowercased).

| Trigger | Tags pushed | Docs |
|---------|-------------|------|
| `push` → `dev` | `nightly`, `{sha7}` | — |
| `release` published (`v1.2.3`) | `v1.2.3`, `latest` | Deploy to GitHub Pages |

## Releases (`_release-draft.yml`)

On `push` → `main`, compares root [`VERSION`](../../VERSION) at HEAD against the latest `v*` tag. When semver increased and `v{VERSION}` does not exist yet, creates the git tag and a **draft** GitHub release. Publishing the release triggers `release.yml` to push stable images.
