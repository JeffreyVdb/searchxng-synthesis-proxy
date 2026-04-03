# ADR 0003: Conventional Commits

**Status:** Accepted

**Date:** 2026-04-03

## Context

Release notes are generated from git commit history. Without a consistent commit format, release notes are noisy and unstructured — mixing features, fixes, chores, and merge commits into a flat list.

## Decision

This repository follows the **Conventional Commits** specification for all commit messages.

### Expected format

```
<type>(<scope>): <subject>

[body]

[BREAKING CHANGE: <description>]
```

### Commit types

| Type | Purpose |
|---|---|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `refactor` | Code restructuring without behavior change |
| `chore` | Maintenance, dependency updates |
| `ci` | CI/CD workflow changes |
| `build` | Build system changes |
| `test` | Test additions or changes |

### Scopes

Scopes are optional but encouraged. Recommended scopes:

- `proxy` — main proxy service
- `mcp` — MCP server
- `api` — HTTP handlers
- `config` — configuration
- `deps` — dependency updates

### Breaking changes

Indicate breaking changes with either:

- A `!` after the type/scope: `feat(api)!: remove deprecated endpoint`
- A `BREAKING CHANGE:` footer

### How GoReleaser uses this

GoReleaser groups changelog entries by commit type:

1. Breaking changes (`^.*?!:`)
2. Features (`^.*?feat(...)`)
3. Fixes (`^.*?fix(...)`)
4. Documentation (`^.*?docs(...)`)
5. Refactors (`^.*?refactor(...)`)
6. Chore/CI/Test (`^.*?(chore|ci|build|test)`)
7. Other (fallback)

Noise commits like `chore(release):` and merge commits are excluded from the changelog.

### Enforcement

Conventional commit format is the documented repository standard. CI enforcement (commitlint, semantic PR titles) is deferred as a follow-up task. If commit quality becomes inconsistent, automated checks can be added later.

## Consequences

- Release notes are structured and scannable.
- Changelog grouping reflects the nature of changes.
- Breaking changes are prominent.
- Contributors are expected to follow the format.
- Enforcement is voluntary initially; automation can be layered in.
