# Review: GoReleaser signing branch (`feature/goreleaser-signing`)

## Verdict

**Not approved yet.**

I found **1 blocking issue** and **3 follow-up issues**. The biggest problem is that the branch currently makes both Go entrypoints fail to compile, which means the new GoReleaser release workflow cannot succeed.

I did **not** find an obvious secret-exposure bug in the GitHub Actions workflow: the minisign private key is materialized into a temp file, passed by path, and cleaned up afterward, which is the right general shape.

## Issues

### 1. Blocking — both binaries fail to compile because declarations were placed before the `import` block

**Files:**
- `cmd/proxy/main.go:4-11`
- `cmd/mcp/main.go:8-15`

**What’s wrong**

The newly added build-metadata variables (`version`, `commit`, `date`) were inserted **before** the `import` block in both `main` packages.

Go source files must have the `import` declarations before other top-level declarations in the file. With the code in its current order, both binaries are syntactically invalid.

**Why it matters**

This breaks the core goal of the branch:

- `go build ./cmd/proxy` fails
- `go build ./cmd/mcp` fails
- GoReleaser cannot build release artifacts for either binary
- the tag-driven release workflow will fail immediately

**Recommended fix**

Move the build-metadata `var` block **below** the `import` block in both files, or put the metadata into a separate Go file in each package.

---

### 2. Medium — release pipeline mutates the source tree during a tagged release

**File:** `.goreleaser.yaml:5-7`

**What’s wrong**

The GoReleaser config runs this hook before every release:

```yaml
before:
  hooks:
    - go mod tidy
```

`go mod tidy` can modify tracked files (`go.mod`, `go.sum`). That is fine in day-to-day development or CI validation, but it is a bad fit for a release pipeline that is supposed to build the exact tagged source.

**Why it matters**

If `go mod tidy` changes module metadata in CI, you get one of two bad outcomes:

1. GoReleaser errors because the repo becomes dirty during release, or
2. the artifacts are built from a tree that no longer exactly matches the tagged commit.

Either outcome makes releases less deterministic.

**Recommended fix**

Remove `go mod tidy` from the release hook. If module hygiene should be enforced, do it in normal CI or pre-merge checks instead.

---

### 3. Medium — ADR 0002 misstates the actual release trigger behavior

**File:** `docs/adr/0002-goreleaser-for-releases.md:46-49`

**What’s wrong**

ADR 0002 says:

> Both the container release workflow and the binary release workflow trigger on `v*` tag pushes.

That is not what the repository currently configures:

- `.github/workflows/release-binaries.yml` triggers on `v*`
- existing `.github/workflows/release.yml` triggers on `v*.*.*`

So the ADR currently documents behavior that is broader than the actual container-release trigger.

**Why it matters**

This is exactly the kind of detail ADRs are supposed to preserve accurately. Someone reading the ADR later will come away with the wrong trigger model.

**Recommended fix**

Either:

- align both workflows to the same tag pattern, **or**
- update the ADR text to match reality (for example: semver release tags trigger both workflows; the binary workflow is currently broader).

---

### 4. Low — README verification snippet is incomplete and can fail for normal users

**File:** `README.md:151-156`

**What’s wrong**

The README tells users to verify releases with:

```bash
minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig
sha256sum -c checksums.txt
```

But `sha256sum -c checksums.txt` only succeeds if **all** listed release artifacts are present in the current directory. A user who downloaded only one archive will get checksum failures for the missing files.

`docs/DEPLOY.md` already documents the nuance and gives a single-file verification example, so the README is currently less accurate than the detailed docs.

**Why it matters**

This creates a frustrating first verification experience even if the release is correct.

**Recommended fix**

Either:

- replace the README snippet with a short pointer to `docs/DEPLOY.md`, or
- add the same caveat/single-file example there.

## Notes

- Secret handling in `release-binaries.yml` looks reasonable overall: the private key is not passed on the command line as raw content, it is written to a temp file with `chmod 600`, and it is cleaned up afterward.
- The archive/signing layout is otherwise in the right direction for GoReleaser: separate archives per binary, checksum generation, and signing only the checksum file are sensible choices.
- I also noticed the archive entries still use the older `builds:` selector under `archives`. Current GoReleaser docs prefer `ids:` there. That looks backwards-compatible rather than broken, so I am not counting it as a review issue, but it would be worth modernizing while touching the config.
