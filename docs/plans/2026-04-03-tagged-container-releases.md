Status: DRAFT

# Plan: Tagged container releases

## Objective

Change the container publishing flow so that:

1. a stable version tag such as `v1.0.0` publishes both `1.0.0` and `latest`; pre-release tags (e.g. `v1.0.0-rc.1`) publish only the semver tag
2. `latest` is no longer overwritten by ordinary pushes to `main`
3. release builds are serialized so two tag pushes cannot race on `latest`
4. the existing path-based change detection on `main` still works
5. tests still run before publishing in both the main-branch and release paths

## Current state

The existing workflow at `.github/workflows/ci-cd.yml` currently does all of the following in one file:

- triggers on pushes to `main`
- triggers on tags matching `v*.*.*`
- runs a `changes` job that skips image builds for docs-only / non-build changes
- runs tests
- builds and pushes a multi-arch image to GHCR
- publishes:
  - `latest` on `main` pushes
  - `{{version}}` on tag pushes

That means `latest` tracks ordinary pushes to `main`, while release tags publish only the versioned image. As a result, `latest` is not a reliable release tag today.

## Recommendation

Split the current mixed-purpose workflow into **two workflows**:

1. **Main CI workflow**: keep the current path-based change detection and test behavior for `main` pushes, but stop publishing `latest` there.
2. **Release workflow**: trigger only on `v*.*.*` tags, always run tests, publish the semver tag, and publish `latest` **only for stable tags** (`vMAJOR.MINOR.PATCH` with no suffix); serialize release builds with `concurrency`.

## Why split instead of keeping one workflow

This is the cleaner separation of concerns once `latest` becomes a release tag:

- `main` pushes are continuous integration events
- `v*.*.*` pushes are release events
- only release events should own `latest`
- release concurrency should apply narrowly to release publishing, not to all CI runs

It also avoids a workflow file full of mutually exclusive `if:` blocks for two different publishing policies.

## Planned file changes

### 1. Update `.github/workflows/ci-cd.yml`

Keep this file for **pushes to `main` only**.

#### Trigger change

Replace the current trigger:

```yaml
on:
  push:
    branches:
      - main
    tags:
      - 'v*.*.*'
```

with:

```yaml
on:
  push:
    branches:
      - main
```

#### Keep the `changes` job

Keep the existing `changes` job so `main` still skips container publishing when only docs or other non-build files changed.

Because tag pushes will move to a separate workflow, remove the tag-specific fast path from the script.

Delete this block from the `changes` job script:

```javascript
const ref = context.ref;

// Release tags are intentional publish events.
if (ref.startsWith('refs/tags/v')) {
  core.setOutput('should_build', 'true');
  core.setOutput('matched_files', '<tag push: forced build>');
  return;
}
```

Everything else in `changes` stays the same, including:

- `compareCommits`
- the build-relevant allowlist
- the initial/force-push fallback

#### Keep the `test` job

Keep the existing `test` job unchanged so `main` pushes still run `go test ./...`.

#### Change the main-branch image tags

The main-branch workflow should stop publishing `latest`.

Update the metadata step from:

```yaml
      - name: Extract image metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest,enable=${{ github.ref == 'refs/heads/main' }}
            type=semver,pattern={{version}}
```

to:

```yaml
      - name: Extract image metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.IMAGE_NAME }}
          flavor: |
            latest=false
          tags: |
            type=raw,value=main
            type=sha,format=short,prefix=sha-
          labels: |
            org.opencontainers.image.title=searchxng-synthesis-proxy
            org.opencontainers.image.description=Search Synthesis Proxy
            org.opencontainers.image.source=https://github.com/JeffreyVdb/searchxng-synthesis-proxy
            org.opencontainers.image.licenses=MIT
```

### Resulting `main` behavior

After this change:

- docs-only pushes to `main` still run tests and skip the image build
- build-relevant pushes to `main` still run tests and publish a preview image
- preview images use `:main` and `:sha-<shortsha>` instead of `:latest`
- `main` can no longer clobber the release `latest` tag

## 2. Add `.github/workflows/release.yml`

Add a second workflow dedicated to tagged releases.

### Recommended full workflow shape

```yaml
name: Release container

on:
  push:
    tags:
      - 'v*.*.*'

permissions:
  contents: read

concurrency:
  group: release-container-latest
  cancel-in-progress: false

env:
  IMAGE_NAME: ghcr.io/jeffreyvdb/searchxng-synthesis-proxy

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Check out source
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run tests
        run: go test ./...

  build-and-push:
    name: Build and push release container
    runs-on: ubuntu-latest
    needs:
      - test
    permissions:
      contents: read
      packages: write
    steps:
      - name: Check out source
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Decide whether this tag should publish latest
        id: latest
        run: |
          if [[ "$GITHUB_REF_NAME" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "publish_latest=true" >> "$GITHUB_OUTPUT"
          else
            echo "publish_latest=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Extract image metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.IMAGE_NAME }}
          flavor: |
            latest=false
          tags: |
            type=semver,pattern={{version}}
            type=raw,value=latest,enable=${{ steps.latest.outputs.publish_latest == 'true' }}
          labels: |
            org.opencontainers.image.title=searchxng-synthesis-proxy
            org.opencontainers.image.description=Search Synthesis Proxy
            org.opencontainers.image.source=https://github.com/JeffreyVdb/searchxng-synthesis-proxy
            org.opencontainers.image.licenses=MIT

      - name: Build and push image
        uses: docker/build-push-action@v6
        with:
          context: .
          file: ./Containerfile
          push: true
          platforms: linux/amd64,linux/arm64
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

## Concurrency design

Use workflow-level concurrency in the release workflow:

```yaml
concurrency:
  group: release-container-latest
  cancel-in-progress: false
```

### Why `cancel-in-progress: false`

Do **not** cancel earlier release runs.

If `v1.0.0` is already building and `v1.0.1` arrives, canceling the first run could mean:

- `1.0.0` never gets published
- the release record exists in git, but the matching image tag does not

Queueing is safer:

- `v1.0.0` finishes and publishes `1.0.0` + `latest`
- `v1.0.1` waits
- `v1.0.1` then publishes `1.0.1` + `latest`
- final `latest` ends up on the newer queued release in the normal oldest-to-newest push order

## Design decision: release workflow does not use the `changes` job

Do **not** carry the path-based change detector into the release workflow.

For releases, the tag itself is the explicit publish signal. The release workflow should:

- always run tests
- always build and publish after tests pass

That keeps release behavior easy to reason about:

- `main` push = test + conditional preview image
- `v*.*.*` tag = test + unconditional release image

## Edge cases

### 1. Docs-only push to `main`

Expected behavior:

- `ci-cd.yml` runs
- `test` runs
- `changes` sets `should_build=false`
- preview image build is skipped
- no impact on `latest`

### 2. Go/code change push to `main`

Expected behavior:

- `ci-cd.yml` runs
- `test` runs
- `changes` sets `should_build=true`
- image publishes with:
  - `main`
  - `sha-<shortsha>`
- no impact on `latest`

### 3. Single release tag push, e.g. `v1.0.0`

Expected behavior:

- `release.yml` runs
- `test` runs
- release image publishes with:
  - `1.0.0`
  - `latest`

### 4. `v1.0.0` and `v1.0.1` pushed within seconds

Expected behavior with the proposed concurrency setup:

- both release runs enter the same concurrency group
- one run builds first
- the other waits until the first completes
- both semver tags are published
- `latest` is updated sequentially instead of racing

This solves the race condition.

### Important nuance

The queued order determines which tag writes `latest` last.

In the normal case where tags are pushed oldest-to-newest, `latest` ends on the newer version.

If someone pushes stable tags out of version order later, `latest` will follow **push order**, not semver order. For example, pushing `v1.0.0` after `v1.0.1` would move `latest` backward.

Pre-release tags (e.g. `v1.0.0-rc.1`) never publish `latest` due to the stable-tag gate, so they cannot interfere with the stable `latest` pointer.

## Stable-tag gate for `latest`

The release workflow includes a gate that prevents pre-release tags from publishing `latest`.

Only tags matching `^v[0-9]+\.[0-9]+\.[0-9]+$` (stable semver, no suffix) publish `latest`. Tags like `v1.0.0-rc.1` or `v1.0.0-beta` still publish the semver tag (e.g. `1.0.0-rc.1`) but do **not** move `latest`.

### Gate step in `release.yml`

```yaml
      - name: Decide whether this tag should publish latest
        id: latest
        run: |
          if [[ "$GITHUB_REF_NAME" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "publish_latest=true" >> "$GITHUB_OUTPUT"
          else
            echo "publish_latest=false" >> "$GITHUB_OUTPUT"
          fi
```

The `latest` tag in the metadata step is conditional on this output:

```yaml
            type=raw,value=latest,enable=${{ steps.latest.outputs.publish_latest == 'true' }}
```

## Validation checklist

### Main workflow validation

1. Push docs-only change to `main`
   - `test` runs
   - build is skipped
   - no `latest` publication

2. Push Go source change to `main`
   - `test` runs
   - image publishes as `main` and `sha-<shortsha>`
   - no `latest` publication

3. Push `Containerfile` or `.github/workflows/ci-cd.yml` change to `main`
   - `changes` still detects it as build-relevant
   - preview image publishes

### Release workflow validation

1. Push `v1.0.0` (stable tag)
   - `test` runs
   - image publishes `1.0.0` and `latest`

2. Push `v1.0.1` (stable tag)
   - `test` runs
   - image publishes `1.0.1` and `latest`

3. Push `v1.0.0-rc.1` (pre-release tag)
   - `test` runs
   - image publishes `1.0.0-rc.1` only
   - `latest` is **not** updated

4. Push two release tags in quick succession
   - second run waits behind the first
   - no concurrent writes to `latest`

## Acceptance criteria

This change is complete when all of the following are true:

- `main` pushes still run tests
- `main` pushes still use the existing path-based build gating
- `main` pushes no longer overwrite `latest`
- `v*.*.*` tag pushes always run tests and then publish the release image
- stable tags (`vMAJOR.MINOR.PATCH`, no suffix) publish both the semver tag and `latest`
- pre-release tags (e.g. `v1.0.0-rc.1`) publish the semver tag but **not** `latest`
- release workflows are serialized with `concurrency`
- two near-simultaneous tag pushes cannot race on `latest`

## Notes for implementation

- Keep the current build steps unchanged where possible; the main changes are trigger separation, tag policy, and release concurrency.
- Reusing the existing `test` job in both workflows via `workflow_call` is possible later, but duplicating the job now is the lowest-risk way to land the release behavior change.
- If downstream consumers already rely on `:latest` meaning "current main branch", document the move to `:main` in the release PR.
