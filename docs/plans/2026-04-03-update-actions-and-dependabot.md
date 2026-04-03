Status: DRAFT

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
   - schedule: every 2 weeks via cron
   - group all action updates into one PR
   - `open-pull-requests-limit: 1`

2. `gomod`
   - directory: `/`
   - schedule: every 2 weeks via cron
   - group all Go module updates into one PR
   - `open-pull-requests-limit: 1`

Recommended config:

```yaml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "cron"
      cronjob: "0 6 */14 * *"
      timezone: "UTC"
    open-pull-requests-limit: 1
    groups:
      github-actions:
        patterns:
          - "*"

  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "cron"
      cronjob: "0 7 8-31/14 * *"
      timezone: "UTC"
    open-pull-requests-limit: 1
    groups:
      gomod:
        patterns:
          - "*"
```

This keeps one grouped PR per ecosystem and staggers the two ecosystems so they do not open on the same run.

## Validation

- Confirm both workflows reference only the exact pinned tags above.
- Confirm `.github/dependabot.yml` passes GitHub validation.
- After merge, confirm Dependabot shows two configured ecosystems and grouped PR behavior.

## Acceptance criteria

- Both workflow files use exact pinned versions for every action in scope.
- `.github/dependabot.yml` exists.
- GitHub Actions updates arrive as one grouped PR.
- Go module updates arrive as one grouped PR.
- Dependabot runs on a biweekly cadence per ecosystem.
