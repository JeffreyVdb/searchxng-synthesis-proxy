Status: DRAFT

# Plan: Fix artifact naming in the CI/CD build pipeline

## Objective

Fix the GoReleaser archive `name_template` so that artifact filenames are clean
(`mcp-server_1.1.1_linux.arm64.tar.gz`) instead of containing errant spaces and
extra underscores (`mcp-server_.1.1.1_.linux_.arm64.tar.gz`).

## Root cause

In `.goreleaser.yaml`, both archive `name_template` values use a YAML **folded
block scalar** (`>-`):

```yaml
name_template: >-
  search-synthesis-proxy_
  {{ .Version }}_
  {{ .Os }}_
  {{ .Arch }}
```

YAML's `>-` folds each newline into a **space**. GoReleaser evaluates the
resulting string as a Go template. Because each template expression sits on its
own line, the folded whitespace is inserted before every expression, producing:

```
search-synthesis-proxy_ 1.1.1_ linux_ amd64
```

This then propagates into:

1. The archive filename on disk — spaces included.
2. The `checksums.txt` file — `sha256sum` records the exact (space-containing)
   filename, so checksum entries inherit the same defect.

GoReleaser's `checksum` section and `signs` section are configured correctly;
they inherit the broken name from the archive artifact.

## Fix

Collapse each `name_template` onto a **single line** so no YAML folding
introduces whitespace. Additionally, replace the last underscore separator
(before `{{ .Arch }}`) with a dot to match the desired convention
`name_version_os.arch.tar.gz`.

### Files to change

| File | What to change |
|------|---------------|
| `.goreleaser.yaml` | Archive `name_template` for both `proxy-archive` and `mcp-archive` |

### Concrete diff

**`proxy-archive`** — change from:

```yaml
    name_template: >-
      search-synthesis-proxy_
      {{ .Version }}_
      {{ .Os }}_
      {{ .Arch }}
```

to:

```yaml
    name_template: "search-synthesis-proxy_{{ .Version }}_{{ .Os }}.{{ .Arch }}"
```

**`mcp-archive`** — change from:

```yaml
    name_template: >-
      mcp-server_
      {{ .Version }}_
      {{ .Os }}_
      {{ .Arch }}
```

to:

```yaml
    name_template: "mcp-server_{{ .Version }}_{{ .Os }}.{{ .Arch }}"
```

### Why this works

* Single-line strings have no folded whitespace — the template evaluates to
  e.g. `mcp-server_1.1.1_linux.arm64`.
* GoReleaser uses this resolved name for:
  * the tar.gz filename
  * the entry written into `checksums.txt`
  * the artifact passed to the minisign signing step
* No other file or workflow needs changes. The `checksum` section
  (`checksums.txt`, sha256) and `signs` section are downstream consumers of the
  archive name and will automatically pick up the corrected filenames.

### Expected output after fix

Filenames:
```
mcp-server_1.1.1_linux.amd64.tar.gz
mcp-server_1.1.1_linux.arm64.tar.gz
search-synthesis-proxy_1.1.1_linux.amd64.tar.gz
search-synthesis-proxy_1.1.1_linux.arm64.tar.gz
```

Checksums (no spaces in filenames):
```
<sha256>  mcp-server_1.1.1_linux.amd64.tar.gz
<sha256>  mcp-server_1.1.1_linux.arm64.tar.gz
<sha256>  search-synthesis-proxy_1.1.1_linux.amd64.tar.gz
<sha256>  search-synthesis-proxy_1.1.1_linux.arm64.tar.gz
```

## Verification

1. Run `goreleaser check` locally to validate the YAML after changes.
2. Dry-run: `goreleaser build --snapshot --clean` and inspect the filenames in
   `dist/` — confirm no errant spaces or extra underscores.
3. Verify `checksums.txt` inside `dist/` has clean filenames.
4. On a real tag push, confirm the GitHub release assets match expected names.

## Out of scope

* Changing the release workflow (`.github/workflows/release-binaries.yml`) — no
  changes needed there.
* Adding or removing supported OS/arch targets.
* Changing the signing or checksum configuration (already correct).
