Status: DRAFT

# Plan: GoReleaser binary releases with conventional commits, SHA256 checksums, and minisign signing

## Objective

Add a tag-driven GitHub release pipeline for this repository that:

1. builds release binaries for both `cmd/proxy/` and `cmd/mcp/`
2. publishes them as GitHub release assets via GoReleaser v2
3. generates release notes from conventional commits, grouped by commit type
4. emits a `checksums.txt` file using SHA256
5. signs the checksum file with minisign and uploads the `.minisig` alongside the artifacts
6. documents the release/signature process and records the architectural decisions in ADRs

## Current state

The repository already has:

- `.github/workflows/ci-cd.yml` for tests + preview container builds on `main`
- `.github/workflows/release.yml` for tagged **container** releases to GHCR
- no `.goreleaser.yaml`
- no GitHub binary release workflow
- no release checksum/signature verification docs
- one existing ADR (`0001-mcp-sse-wrapper.md`), so the requested ADRs should continue from there as `0002` and `0003`

Important repo-specific nuance: the existing `release.yml` is already a tag-triggered workflow for containers. The binary release plan should **coexist** with that flow instead of silently replacing it.

## Recommendation

Implement the binary release pipeline as a **separate, dedicated workflow** that runs alongside the existing container release workflow.

### Why this is the right shape

- It satisfies the request without regressing the current GHCR tag-release automation.
- It keeps responsibilities clear:
  - existing `release.yml` = container image release
  - new binary release workflow = GitHub release artifacts via GoReleaser
- It avoids forcing GoReleaser into container responsibilities that are already working.
- It keeps the binary release pipeline focused on:
  - archives
  - changelog generation
  - checksums
  - minisign signing
  - GitHub release asset publishing

## Scope of changes

### New files

1. `.goreleaser.yaml`
2. `.github/workflows/release-binaries.yml` (recommended name)
3. `docs/adr/0002-goreleaser-for-releases.md`
4. `docs/adr/0003-conventional-commits.md`
5. `minisign.pub` (recommended repository-root public key file)

### Updated files

1. `README.md`
2. `docs/DEPLOY.md`

### Optional follow-up file(s)

Not strictly required for the requested release pipeline, but worth considering if we want automatic commit-format enforcement immediately:

- `.github/workflows/conventional-commits.yml`
- `commitlint.config.cjs` or a semantic-PR-title check configuration

I would keep those as a small follow-up unless the implementation pass explicitly wants enforcement in the same PR.

## Planned implementation details

## 1. Add GoReleaser v2 configuration

Create `.goreleaser.yaml` at the repository root and declare `version: 2`.

### Build layout

Configure two `builds` entries:

- `proxy` → `./cmd/proxy`
- `mcp` → `./cmd/mcp`

Recommended binary names:

- `search-synthesis-proxy`
- `mcp-server`

These match the current documentation and avoid unnecessary deployment churn.

### Platform scope

Recommended initial release targets:

- `linux/amd64`
- `linux/arm64`

Reasoning:

- current deployment docs are Linux-focused
- existing container releases already target Linux amd64/arm64
- this keeps the first binary release implementation small and predictable

If desktop distribution becomes important later, GoReleaser can be extended to add `darwin` and `windows` targets without reworking the pipeline.

### Archive layout

Produce separate archives per binary/platform, for example:

- `search-synthesis-proxy_v1.2.3_linux_amd64.tar.gz`
- `mcp-server_v1.2.3_linux_arm64.tar.gz`

Use `zip` for Windows if Windows targets are added later.

### Version metadata

Use GoReleaser `ldflags` to inject version metadata into both binaries if the `main` packages expose version variables. If they do not yet, implementation can either:

1. add those variables as a small supporting change, or
2. defer embedded version reporting and keep the first release plan focused on packaging/signing

This is nice-to-have, not a blocker for the requested release pipeline.

## 2. Generate SHA256 checksums

Use GoReleaser’s `checksum` section with:

- `algorithm: sha256`
- `name_template: checksums.txt`

This keeps the output filename exactly aligned with the requirement and easy to document.

The generated `checksums.txt` should be uploaded as a GitHub release asset by GoReleaser.

## 3. Sign the checksum file with minisign

Use GoReleaser’s `signs` section and sign **only the checksum file**.

### Why sign checksums instead of every artifact

Signing `checksums.txt` is the simplest and most standard approach for this repository because:

- users verify one signature instead of many
- the signed checksum file authenticates the full artifact set
- it keeps the release smaller and the verification instructions simpler
- it directly satisfies the user’s requirement

### GoReleaser signing approach

Use a signing block conceptually like this:

```yaml
signs:
  - artifacts: checksum
    cmd: minisign
    signature: "${artifact}.minisig"
    args:
      - "-S"
      - "-s"
      - "{{ .Env.MINISIGN_KEY_FILE }}"
      - "-m"
      - "${artifact}"
      - "-x"
      - "${signature}"
      - "-t"
      - "{{ .ProjectName }} {{ .Version }} checksums"
```

That yields:

- `checksums.txt`
- `checksums.txt.minisig`

Both should be attached to the GitHub release.

## 4. Minisign key management design

This is the most important implementation decision in the plan.

### Constraint

The request names a single GitHub Actions secret: `MINISIGN_PRIVATE_KEY`.

Minisign’s default secret-key flow is password-protected and interactive, which is awkward for GitHub Actions unless a second secret/passphrase mechanism is introduced.

### Recommended solution

Generate a **dedicated CI signing keypair** using a passwordless minisign secret key.

That means the one-time implementation step should use `minisign -G -W` (or equivalent password-removal flow) so that CI can sign non-interactively.

### Why this tradeoff is acceptable here

- the private key will still be stored in GitHub Secrets, not in the repo
- the user explicitly wants to store the private key in a password manager
- the requirement only names one CI secret, so a passwordless CI key avoids inventing a second required secret
- this key is only for release signing, not for broader infrastructure access

### Operational handling

During implementation:

1. generate the minisign keypair locally/offline
2. save the **private key text** in the user’s password manager
3. add the same private key text to the repository secret `MINISIGN_PRIVATE_KEY`
4. commit the public key to the repo as `minisign.pub`
5. document the public key in `docs/DEPLOY.md`

### Workflow handling

The GitHub Actions workflow should:

1. read `${{ secrets.MINISIGN_PRIVATE_KEY }}`
2. write it to a temporary file, e.g. `$RUNNER_TEMP/minisign.key`
3. `chmod 600` that file
4. export `MINISIGN_KEY_FILE` for the GoReleaser step

This avoids trying to pass the key inline on the command line.

## 5. Add a dedicated GitHub Actions binary-release workflow

Create `.github/workflows/release-binaries.yml`.

### Trigger

Trigger on tag pushes, preferably:

```yaml
on:
  push:
    tags:
      - 'v*'
```

Using `v*` keeps the workflow compatible with both stable and pre-release semver tags.

### Permissions

Use:

```yaml
permissions:
  contents: write
```

GoReleaser needs this to create/update the GitHub release and upload assets.

### Checkout requirements

Use `actions/checkout` with:

```yaml
with:
  fetch-depth: 0
```

This is required so GoReleaser can see tags/history and generate changelogs correctly.

### Workflow steps

Recommended shape:

1. check out source with full history
2. set up Go from `go.mod`
3. install `minisign`
4. materialize `MINISIGN_PRIVATE_KEY` into a temporary file
5. run `goreleaser/goreleaser-action@v7` with:
   - `version: '~> v2'`
   - `args: release --clean`

### Environment for GoReleaser step

Provide at least:

- `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
- `MINISIGN_KEY_FILE: ${{ env.MINISIGN_KEY_FILE }}`

### Concurrency

Add workflow-level concurrency, for example:

```yaml
concurrency:
  group: goreleaser-release
  cancel-in-progress: false
```

This is not strictly required, but it is a sensible guard so two rapid tag pushes do not attempt overlapping release writes.

## 6. Configure changelog generation from conventional commits

Use GoReleaser’s built-in `changelog` block with `use: git`.

### Why `git` instead of `github-native`

`github-native` would generate release notes via GitHub, but it disables grouping/filtering controls.

The user explicitly wants changelog entries grouped by conventional commit type, so `github-native` is the wrong fit.

`use: git` gives predictable grouping behavior as long as the workflow fetches full history.

### Recommended group structure

Group commit subjects into sections such as:

1. Breaking changes
2. Features
3. Fixes
4. Docs
5. Refactors
6. Chore / CI / Test
7. Others

Representative regex patterns:

- Breaking changes: `^.*?!:`
- Features: `^.*?feat(\([[:word:]-]+\))?!?:`
- Fixes: `^.*?fix(\([[:word:]-]+\))?!?:`
- Docs: `^.*?docs(\([[:word:]-]+\))?!?:`
- Refactors: `^.*?refactor(\([[:word:]-]+\))?!?:`
- Chore/CI/Test: `^.*?(chore|ci|build|test)(\([[:word:]-]+\))?!?:`
- Others: fallback group without a regex

### Filters

Exclude release-noise commits if needed, for example:

- `^chore\(release\):`
- merge boilerplate if it appears in history

### Enforcement note

GoReleaser can only group what is already in git history. That means the ADR should explicitly document the expected conventional commit format and the repo should at least document that contributors are expected to use it.

Hard CI enforcement is optional for this task, but the ADR should be clear about the intended standard.

## 7. ADR 0002: GoReleaser for releases

Create `docs/adr/0002-goreleaser-for-releases.md`.

It should cover:

- why GoReleaser was chosen over hand-written shell scripts or a fully custom GitHub Actions release pipeline
- why binary releases are separate from the existing container workflow
- why checksums are generated by GoReleaser
- why only `checksums.txt` is minisign-signed
- why the initial binary release target set is Linux amd64/arm64
- how the workflow uses a tag push as the release trigger
- how the private key is supplied at runtime via `MINISIGN_PRIVATE_KEY`

## 8. ADR 0003: Conventional commits

Create `docs/adr/0003-conventional-commits.md`.

It should cover:

- why release notes are derived from commit history
- expected commit format (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `ci:`, `test:`)
- how scopes should be used
- how breaking changes are represented (`!` and/or `BREAKING CHANGE:`)
- how GoReleaser groups changelog entries from these commit subjects
- what enforcement policy the repo intends to follow

Recommended ADR position:

- document conventional commits as the repository standard immediately
- optionally add automation later if commit quality becomes inconsistent

## 9. Documentation updates

### `docs/DEPLOY.md`

Add a new release/distribution section covering:

1. how maintainers cut a release
   - ensure commits are merged
   - create and push a semver tag
   - GitHub Actions runs GoReleaser
   - release assets appear on the GitHub release
2. what artifacts are expected
   - proxy archive(s)
   - MCP archive(s)
   - `checksums.txt`
   - `checksums.txt.minisig`
3. how to verify a release
   - download asset(s)
   - download `checksums.txt`
   - download `checksums.txt.minisig`
   - obtain `minisign.pub`
   - verify minisign signature first
   - verify SHA256 checksum second

### Verification examples to document

Recommended docs flow:

```bash
minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig
sha256sum -c checksums.txt
```

Because `sha256sum -c` checks every listed asset in the current directory, the docs should be explicit about either:

- downloading the full artifact set before running it, or
- grepping the relevant line for a single file before verification

For a single-file verification example, document something like:

```bash
grep 'search-synthesis-proxy_.*_linux_amd64.tar.gz' checksums.txt | sha256sum -c -
```

### Public key documentation

Include both:

- the `minisign.pub` file in the repo
- the public key contents or a clearly copyable block in `docs/DEPLOY.md`

That gives users two ways to obtain the verification key.

### `README.md`

Add a short release-verification note in the documentation section or near installation/deployment links, for example:

- releases include SHA256 checksums and a minisign signature
- verification instructions live in `docs/DEPLOY.md`
- public verification key is stored in `minisign.pub`

## 10. Validation plan

Before shipping the implementation, validate with:

### Local validation

1. `goreleaser check`
2. `goreleaser release --snapshot --clean`
3. confirm `dist/` contains:
   - archives for both binaries
   - `checksums.txt`
   - `checksums.txt.minisig`
4. run minisign verification locally against the generated checksum file
5. run checksum verification against at least one produced artifact

### GitHub Actions validation

Use a test/pre-release tag first if desired (for example `v0.0.1-rc.1`) and verify:

1. the workflow runs on tag push
2. the GitHub release is created
3. grouped release notes are populated from commit history
4. the checksum and `.minisig` are attached
5. the public key instructions match the actual generated key

## Risks and edge cases

### 1. Existing tagged container workflow also runs on the same tag

This is expected.

A tag push will trigger:

- the existing container release workflow
- the new GoReleaser binary release workflow

That is acceptable as long as the two workflows have distinct responsibilities and names.

### 2. Conventional-commit quality determines changelog quality

If the recent commit history is inconsistent, the first release notes may have a noisier “Others” section.

That is not a blocker, but it should be acknowledged in the ADR.

### 3. Passwordless CI signing key is a deliberate tradeoff

The single-secret requirement strongly suggests a passwordless minisign secret key for CI.

This is acceptable, but the docs/ADR should make it explicit that:

- the key is release-specific
- it must never be committed to the repository
- it should be stored in GitHub Secrets and the maintainer’s password manager only

## Acceptance criteria

This plan is complete when the implementation produces all of the following:

- a GoReleaser v2 config that builds both `cmd/proxy` and `cmd/mcp`
- a tag-triggered GitHub Actions workflow that runs GoReleaser `release`
- a GitHub release with binary assets for both programs
- release notes generated from conventional commits and grouped by type
- a `checksums.txt` asset using SHA256
- a `checksums.txt.minisig` asset generated with minisign
- public verification key committed to the repository
- `docs/adr/0002-goreleaser-for-releases.md`
- `docs/adr/0003-conventional-commits.md`
- updated `docs/DEPLOY.md` with release and verification steps
- updated `README.md` pointing users to verification docs

## Suggested implementation order

1. add `.goreleaser.yaml`
2. add `release-binaries.yml`
3. generate minisign keypair and store secrets/public key
4. update docs (`README.md`, `docs/DEPLOY.md`)
5. write ADR 0002 and ADR 0003
6. validate locally with `goreleaser check` / `--snapshot`
7. validate in GitHub with a test tag
