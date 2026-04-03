Status: COMPLETED

# Plan: Update GitHub Actions and add Dependabot

## Objective

Refresh the repo’s pinned GitHub Actions to current exact release tags and add a Dependabot config that keeps workflow actions and Go modules updated in grouped, low-noise PRs.

## Exact versions to pin

Latest tags verified from GitHub release/tag data on 2026-04-03:

- `actions/checkout` → `v6.0.2`
- `actions/setup-go` → `v6.4.0`
- `actions/github-script` → `v8.0.0`
- `docker/setup-qemu-action` → `v4.0.0`
- `docker/setup-buildx-action` → `v4.0.0`
- `docker/login-action` → `v4.1.0`
- `docker/metadata-action` → `v6.0.0`
- `docker/build-push-action` → `v7.0.0`

## Planned changes

### 1. Update workflow pins

Update both workflow files:

- `.github/workflows/ci-cd.yml`
- `.github/workflows/release.yml`

Replace all current floating major pins with the exact versions above. No workflow logic changes are needed beyond version bumps.

### 2. Add `.github/dependabot.yml`

Create a Dependabot config with two update entries:

1. `github-actions`
   - directory: `/`
   - schedule: weekly (Monday), with grouping limiting to one open PR per ecosystem
   - group all action updates into one PR
   - `open-pull-requests-limit: 1`

2. `gomod`
   - directory: `/`
   - schedule: weekly (Monday), with grouping limiting to one open PR per ecosystem
   - group all Go module updates into one PR
   - `open-pull-requests-limit: 1`

Recommended config:

```yaml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
    open-pull-requests-limit: 1
    groups:
      github-actions:
        patterns:
          - "*"

  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
    open-pull-requests-limit: 1
    groups:
      gomod:
        patterns:
          - "*"
```

This keeps one grouped PR per ecosystem. Dependabot checks weekly on Monday, but with `open-pull-requests-limit: 1` and grouping, at most one PR per ecosystem stays open at a time.

## Validation

- Confirm both workflows reference only the exact pinned tags above.
- Confirm `.github/dependabot.yml` passes GitHub validation.
- After merge, confirm Dependabot shows two configured ecosystems and grouped PR behavior.

## Acceptance criteria

- Both workflow files use exact pinned versions for every action in scope.
- `.github/dependabot.yml` exists.
- GitHub Actions updates arrive as one grouped PR.
- Go module updates arrive as one grouped PR.
- Dependabot runs on a weekly cadence (Monday) per ecosystem, with grouping limiting to one open PR per ecosystem.
