Status: COMPLETED

# Plan: Path-based CI triggers for container builds

## Objective

Reduce unnecessary container builds in GitHub Actions while preserving fast feedback from tests.

## Problem statement

The current workflow at `.github/workflows/ci-cd.yml` runs both jobs on:

- every push to `main`
- every version tag matching `v*.*.*`

That means non-build changes such as `LICENSE`, `README.md`, or `docs/` updates still trigger a full multi-arch container build and push. This wastes CI minutes and registry/build cache churn for changes that cannot affect the binary or container image.

## Required end state

1. **Tests still run on every push** covered by the workflow.
2. **`build-and-push` only runs when build-relevant files changed**, specifically:
   - any `*.go` file
   - `go.mod`
   - `go.sum`
   - `Containerfile`
   - `.dockerignore`
   - anything under `.github/workflows/`
3. Documentation-only and metadata-only changes such as the following must **not** trigger `build-and-push`:
   - `docs/**`
   - `LICENSE`
   - `README.md`
   - `.gitignore`
   - similar non-runtime/non-image files

## Recommended design

Keep a **single workflow** and add a lightweight **change-detection job** that decides whether the container build is needed.

### Why this design

- Using `on.push.paths` at the workflow level would skip the **entire** workflow, including tests, which violates the requirement that tests run on every push.
- A job-level gate keeps test execution unchanged while making the container build conditional.
- Keeping everything in one workflow is the smallest, clearest change to the existing CI/CD setup.

## Design decision: tag behavior

Version tags should remain an **explicit release signal**.

### Decision

- **Branch pushes**: build only when relevant files changed.
- **Version tag pushes (`refs/tags/v*.*.*`)**: allow `build-and-push` to run unconditionally.

### Rationale

GitHub Actions path filters are not reliable as a sole solution for tag pushes, and version tags are typically an intentional release event. This keeps semver image publishing working while still eliminating the wasteful builds caused by docs-only or license-only pushes to `main`.

If strict path filtering for tags is desired later, that should be treated as a follow-up enhancement because it requires separate comparison logic for tag refs.

## Implementation plan

### 1. Keep the workflow trigger broad enough for tests

Do **not** add workflow-level `paths` filters to the `push` trigger.

The top-level trigger should remain conceptually equivalent to:

```yaml
on:
  push:
    branches:
      - main
    tags:
      - 'v*.*.*'
```

This ensures the workflow still starts for every relevant push so the test job always runs.

### 2. Add a new `changes` job before `build-and-push`

Add a job named something like `changes` or `detect-build-changes`.

#### Responsibilities

- Determine whether the current push includes any build-relevant file changes.
- Emit a job output such as `should_build=true|false`.
- Treat version tags as `should_build=true` immediately.

#### Recommended implementation

Use `actions/github-script` to compare the pushed commit range on branch pushes.

This avoids depending on workflow-level path filters and keeps the logic explicit and inspectable.

#### Target job shape

```yaml
jobs:
  changes:
    name: Detect build-relevant changes
    runs-on: ubuntu-latest
    permissions:
      contents: read
    outputs:
      should_build: ${{ steps.detect.outputs.should_build }}
      matched_files: ${{ steps.detect.outputs.matched_files }}
    steps:
      - name: Detect whether build should run
        id: detect
        uses: actions/github-script@v7
        with:
          script: |
            const ref = context.ref;

            // Release tags are intentional publish events.
            if (ref.startsWith('refs/tags/v')) {
              core.setOutput('should_build', 'true');
              core.setOutput('matched_files', '<tag push: forced build>');
              return;
            }

            const before = context.payload.before;
            const after = context.sha;

            const response = await github.rest.repos.compareCommits({
              owner: context.repo.owner,
              repo: context.repo.repo,
              base: before,
              head: after,
            });

            const files = (response.data.files || []).map(file => file.filename);

            const matched = files.filter(file =>
              file.endsWith('.go') ||
              file === 'go.mod' ||
              file === 'go.sum' ||
              file === 'Containerfile' ||
              file === '.dockerignore' ||
              file.startsWith('.github/workflows/')
            );

            core.info(`Changed files: ${files.join(', ') || '<none>'}`);
            core.info(`Matched build files: ${matched.join(', ') || '<none>'}`);

            core.setOutput('matched_files', matched.join(', '));
            core.setOutput('should_build', matched.length > 0 ? 'true' : 'false');
```

### 3. Leave the `test` job functionally unchanged

The `test` job should continue to run whenever the workflow is triggered.

No path filtering should be applied to the test job.

The existing test job can remain as-is except for any optional cleanup or YAML reordering.

### 4. Gate `build-and-push` on the `changes` job output

Update `build-and-push` so it depends on both `test` and `changes`, and only runs when `should_build == 'true'`.

#### Target condition

```yaml
  build-and-push:
    name: Build and push container
    runs-on: ubuntu-latest
    needs:
      - test
      - changes
    if: needs.changes.outputs.should_build == 'true'
```

### 5. Preserve the existing build logic inside `build-and-push`

Do not change the current container build steps unless required for wiring in the new condition.

Keep these behaviors unchanged:

- checkout
- QEMU setup
- Buildx setup
- GHCR login
- metadata extraction
- multi-arch build/push
- cache configuration

This change is about **when** the job runs, not **how** the image is built.

## Files that must be considered build-relevant

The detection logic should treat these as build-relevant:

```text
**/*.go
go.mod
go.sum
Containerfile
.dockerignore
.github/workflows/**
```

## Files that must not trigger a build by themselves

Examples:

```text
docs/**
LICENSE
README.md
.gitignore
```

This does not require an explicit denylist if the implementation uses the allowlist above.

## Validation checklist

After implementation, validate these cases:

### Should run tests only; skip build

1. Change only `LICENSE` and push to `main`
2. Change only `README.md` and push to `main`
3. Change only files under `docs/` and push to `main`
4. Change only `.gitignore` and push to `main`

Expected result in each case:

- workflow starts
- `test` runs and passes/fails normally
- `changes` reports `should_build=false`
- `build-and-push` is skipped

### Should run tests and build

1. Change any Go source file and push to `main`
2. Change `go.mod` or `go.sum` and push to `main`
3. Change `Containerfile` and push to `main`
4. Change `.dockerignore` and push to `main`
5. Change `.github/workflows/ci-cd.yml` and push to `main`

Expected result in each case:

- workflow starts
- `test` runs
- `changes` reports `should_build=true`
- `build-and-push` runs

### Tag validation

1. Push a version tag such as `v0.1.0`

Expected result:

- workflow starts
- `changes` reports `should_build=true` because tag pushes are treated as explicit release builds
- `build-and-push` runs and publishes the semver-tagged image

## Acceptance criteria

The implementation is complete when all of the following are true:

- A `main` push that only touches `LICENSE`, `README.md`, `.gitignore`, or `docs/**` does **not** execute `build-and-push`
- A `main` push that touches Go code, Go module files, container build files, or workflow files **does** execute `build-and-push`
- The `test` job still runs on every workflow invocation
- Version tag pushes still publish images
- The workflow remains easy to understand from the YAML alone

## Notes for the implementing coder

- Prefer minimal YAML churn: add one detection job and one `if` condition rather than restructuring the entire workflow.
- Keep the allowlist centralized in one place so future build-relevant files can be added without hunting through multiple conditions.
- If you want extra visibility, expose the matched files in the workflow summary or logs, but that is optional.
