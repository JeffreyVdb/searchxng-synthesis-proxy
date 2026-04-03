# Review: tagged container releases

## Summary

I reviewed all changes on `feature/tagged-container-releases` relative to `main`:

- `.github/workflows/ci-cd.yml`
- `.github/workflows/release.yml`
- `docs/plans/2026-04-03-tagged-container-releases.md`

The split between mainline preview builds and tagged release builds is the right overall direction, and the YAML shape looks consistent. I did find two issues that should be fixed before merging.

---

## 1) Pre-release tags can incorrectly overwrite `latest`

- **Severity:** Medium
- **File:** `.github/workflows/release.yml`
- **Lines:** 4-6, 69-71

### Current code

```yaml
on:
  push:
    tags:
      - 'v*.*.*'
```

```yaml
tags: |
  type=semver,pattern={{version}}
  type=raw,value=latest
```

### Why this is a problem

The workflow trigger uses a broad glob (`v*.*.*`), and the workflow unconditionally adds `latest` for every matching tag.

That means tags such as `v1.2.3-rc.1` or other non-stable variants that still match the glob can end up publishing `latest`.

This is exactly the case that `docker/metadata-action` normally avoids when you rely on `type=semver` alone: pre-release tags only emit the versioned pre-release tag, not `latest`. The added raw `latest` tag bypasses that safety.

### Impact

A pre-release or malformed release tag could move the public `latest` image away from the current stable release.

### Fix

Gate `latest` so it is only published for stable tags that match the intended release format, for example `vMAJOR.MINOR.PATCH` with no suffix.

One straightforward fix is to add a small step before metadata extraction:

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

Then update metadata generation:

```yaml
tags: |
  type=semver,pattern={{version}}
  type=raw,value=latest,enable=${{ steps.latest.outputs.publish_latest == 'true' }}
```

If you want `latest` to mean "highest stable semver" rather than "last pushed stable tag," add the existing plan’s higher-version gate as a follow-up.

---

## 2) The plan document describes the old workflow incorrectly

- **Severity:** Low
- **File:** `docs/plans/2026-04-03-tagged-container-releases.md`
- **Lines:** 24-29

### Current text

```md
- publishes:
  - `latest` on `main` pushes
  - `{{version}}` on tag pushes

That means a clean release tag can publish `latest`, but any later push to `main` can immediately replace it.
```

### Why this is a problem

That last sentence does not match the workflow it is describing.

In the original `ci-cd.yml`, `latest` was only enabled when `github.ref == 'refs/heads/main'`, so tag pushes did **not** publish `latest`. Tag pushes only published the semver tag.

### Impact

The plan’s problem statement is misleading for anyone trying to understand why this change was needed or how the old behavior actually worked.

### Fix

Rewrite that sentence so it accurately reflects the previous workflow behavior. For example:

```md
That means `latest` tracks ordinary pushes to `main`, while release tags publish only the versioned image. As a result, `latest` is not a reliable release tag today.
```

---

## Final assessment

The branch is close, but I would **not merge it yet** because the new release workflow can allow pre-release tags to overwrite `latest`. After that is fixed, the workflow split itself looks sound.
