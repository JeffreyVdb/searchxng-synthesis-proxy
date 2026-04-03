# ADR 0002: GoReleaser for Binary Releases

**Status:** Accepted

**Date:** 2026-04-03

## Context

The repository needed a tag-driven release pipeline that produces GitHub release binaries, generates SHA256 checksums, signs the checksums with minisign, and produces grouped release notes from conventional commits.

Several approaches were considered:

1. **Hand-written shell scripts** in a custom GitHub Actions workflow — full control, but high maintenance surface and easy to get wrong (cross-compilation, archive naming, checksum generation).
2. **Fully custom GitHub Actions pipeline** using individual actions for each step — composable but verbose; GoReleaser already encapsulates this.
3. **GoReleaser v2** — purpose-built for Go binary releases with first-class support for builds, archives, checksums, signing, changelogs, and GitHub release publishing.

## Decision

Use **GoReleaser v2** to handle the entire binary release lifecycle: build, archive, checksum, sign, changelog generation, and GitHub release publishing.

### Separate workflow from container releases

The existing `release.yml` handles tagged container image releases to GHCR. Binary releases run as a **separate, dedicated workflow** (`release-binaries.yml`). This keeps responsibilities clear and avoids disrupting the working container pipeline.

### Binary targets

Initial release targets are Linux amd64 and arm64, matching the platforms already supported by container builds. Desktop platforms (darwin, windows) can be added later without reworking the pipeline.

### Checksums

GoReleaser's built-in `checksum` section produces `checksums.txt` with SHA256 digests. This is the standard, well-understood approach for artifact integrity verification.

### Signing

Only `checksums.txt` is signed with minisign — not individual artifacts. This is the simplest and most standard approach because:

- Users verify one signature instead of many.
- The signed checksum file authenticates the full artifact set.
- Release output stays small.
- Verification instructions stay simple.

### Key management

A dedicated CI signing keypair is used with a passwordless secret key, stored as the `MINISIGN_PRIVATE_KEY` GitHub secret. The public key is committed as `minisign.pub`. The workflow materializes the private key into a temporary file at runtime and cleans it up after signing.

### Trigger

Both the container release workflow and the binary release workflow trigger on `v*` tag pushes. This is expected and desirable — one tag produces both a container image and binary release assets.

## Consequences

- GoReleaser manages cross-compilation, archive layout, and release asset publishing.
- Adding new platforms or binaries requires only `.goreleaser.yaml` changes.
- The minisign private key must remain in GitHub Secrets and the maintainer's password manager only.
- Release quality depends on conventional commit discipline (see ADR 0003).
