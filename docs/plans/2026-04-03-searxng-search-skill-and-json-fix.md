Status: COMPLETED

# Plan: Fix LLM JSON parsing and add the `searxng-search` OpenClaw skill

## Objective

Ship two related improvements:

1. Harden the proxy against slightly malformed LLM output so `llm_invalid_json` becomes much rarer in real usage.
2. Add a reusable OpenClaw skill that queries the deployed synthesis proxy at `https://synth.vandenborne.co` with HTTP Basic Auth and returns a clean answer + cited sources.

## What exists today

### Proxy codebase findings

After reviewing the current code in `internal/proxy/prompt.go` and `internal/proxy/service.go`:

- `stripFences()` uses one regex and only cleanly handles the simplest wrapper shape: a fully fenced block with optional lowercase `json`.
- `parseSynthesisJSON()` only tries one `json.Unmarshal` after `stripFences(strings.TrimSpace(raw))`.
- If the LLM returns a short preamble, uppercase fence tag, CRLF line endings, BOM-prefixed text, or prose surrounding the JSON object, parsing fails immediately and the API returns `llm_invalid_json`.
- Prompt-related tests currently live inside `internal/proxy/service_test.go`, so prompt parsing coverage is present but not organized around the failure mode that needs fixing.

### Deployment/skill findings

- The live endpoint is `https://synth.vandenborne.co`.
- A direct unauthenticated fetch to `/healthz` currently returns `401 Unauthorized`, so the skill and its QA checks should send Basic Auth consistently instead of assuming healthz is public.
- `curl` is not available in the current sandbox, while `python3` is. For portability, the skill should use a bundled Python helper executed via `exec`, not assume `curl` exists.

## Scope split

This work spans two roots:

1. **Repo-tracked code**: `/home/node/.openclaw/workspace/searchxng-synthesis-proxy`
2. **Workspace skill**: `/home/node/.openclaw/workspace/skills/searxng-search`

The feature branch in this repo will track the plan and later the Go-side JSON fix. The skill itself lives in the workspace skills directory and should be created there during implementation.

## Task 1: Fix the LLM invalid JSON bug

### Files to change

- `internal/proxy/prompt.go`
- `internal/proxy/prompt_test.go` **(new)**
- `internal/proxy/service_test.go` **(trim prompt-specific tests out if they are moved to `prompt_test.go`)**

### Implementation design

#### 1. Replace the fragile fence regex with conservative wrapper stripping

Keep the public behavior the same:

```go
func stripFences(s string) string
```

But change the implementation from a single regex to a small parser that:

1. Removes leading UTF-8 BOM (`\ufeff`) and outer whitespace.
2. Accepts leading/trailing blank lines.
3. Recognizes both backtick and tilde fences.
4. Accepts fence lengths of 3 or more characters.
5. Accepts optional info strings such as:
   - ````json````
   - ````JSON````
   - ```` json````
   - `~~~json`
6. Handles both LF and CRLF line endings.
7. Only unwraps when the entire response is a single fenced block.
8. Returns the original string unchanged when the wrapper is incomplete or ambiguous.

Recommended approach: parse by lines rather than by regex. Regex is the current source of brittleness.

A good internal helper layout is:

```go
func stripFences(s string) string
func trimLLMOutput(s string) string
```

`trimLLMOutput` should remove BOM + whitespace so the same normalization is applied before and after fence stripping.

#### 2. Add fallback extraction of the first JSON object

Add a helper in `internal/proxy/prompt.go`:

```go
func extractFirstJSONObject(s string) (string, bool)
```

Implementation requirements:

- Find the first `{`.
- Scan forward until the matching `}` for that object.
- Track nesting depth.
- Ignore braces that appear inside quoted JSON strings.
- Handle escaped quotes (`\"`) correctly so the string-state tracking does not break.
- Return `(substring, true)` only when a balanced object is found.
- Return `("", false)` when no balanced object exists.

This is still a “simple brace matcher”, but it must be string-aware. Otherwise valid answers such as `{"answer":"use map[string]int { ... }","citations":[1]}` will break the matcher.

#### 3. Retry parsing with the extracted JSON object

Keep the existing signature:

```go
func parseSynthesisJSON(raw string) (synthesisPayload, error)
```

Refactor the internals to follow this order:

1. `cleaned := stripFences(raw)`
2. Try to unmarshal `cleaned`
3. If that fails, call `extractFirstJSONObject(cleaned)`
4. If extraction succeeds, try to unmarshal the extracted substring
5. Validate `payload.Answer` exactly as today (`strings.TrimSpace`, non-empty)
6. Return the original invalid-JSON style error if all parse attempts fail

Recommended helper to reduce duplication:

```go
func decodeSynthesisPayload(raw string) (synthesisPayload, error)
```

Then `parseSynthesisJSON` becomes orchestration: normalize → try decode → fallback extract → validate.

#### 4. Tighten the system prompt

Update `systemPrompt` in `internal/proxy/prompt.go` so the LLM has fewer excuses to add prose or markdown.

Add explicit rules along these lines:

- “Your entire response must be exactly one JSON object.”
- “Do not include code fences, markdown, commentary, preambles, or trailing text.”
- “Do not include any keys other than `answer` and `citations`.”
- “If the evidence is weak, still return valid JSON and say that in `answer`.”
- “Never wrap the JSON in backticks.”

Keep the current safety constraints about using only the provided sources.

### Test plan for Task 1

Create `internal/proxy/prompt_test.go` and move prompt-parsing tests there.

#### Target test functions

```go
func TestStripFences(t *testing.T)
func TestExtractFirstJSONObject(t *testing.T)
func TestParseSynthesisJSON(t *testing.T)
```

#### Required `stripFences` cases

At minimum cover:

1. Plain JSON is unchanged
2. Triple-backtick fenced JSON
3. Triple-backtick fenced JSON with trailing whitespace
4. Triple-backtick fenced JSON with CRLF line endings
5. Uppercase fence tag: ` ```JSON `
6. Space before info string: ` ``` json `
7. Tilde fence: `~~~json`
8. Leading BOM before fence
9. Leading blank lines before fence and trailing blank lines after closing fence
10. Incomplete/unmatched fence returns original input unchanged

#### Required `extractFirstJSONObject` cases

At minimum cover:

1. Pure JSON object
2. Preamble + JSON + trailing note
3. Nested objects/arrays
4. Braces inside strings
5. Escaped quotes inside strings
6. No object present
7. Unbalanced object

#### Required `parseSynthesisJSON` cases

At minimum cover:

1. Clean JSON parses
2. Fenced JSON parses
3. BOM + fenced JSON parses
4. Prose-wrapped JSON parses via fallback extraction
5. Fenced JSON with extra prose outside the fence still parses via fallback extraction
6. Empty answer after trimming returns `missing answer field`
7. Completely non-JSON text still returns invalid JSON error

Add one end-to-end service-level regression in `internal/proxy/service_test.go` if helpful:

```go
func TestService_EmbeddedJSONObject(t *testing.T)
```

This should use a generator response such as:

    Here is the result:
    ```json
    {"answer":"ok","citations":[1]}
    ```

and verify the service returns a normal 200-path response rather than `llm_invalid_json`.

### Acceptance criteria for Task 1

- Responses wrapped in common markdown fence variants no longer fail parsing.
- Responses with short surrounding prose parse successfully when they still contain one valid top-level JSON object.
- Existing payload validation still rejects empty `answer` values.
- `go test ./internal/proxy` passes.
- `go test ./...` still passes.

## Task 2: Create the `searxng-search` OpenClaw skill

Follow the `skill-creator` process explicitly.

### Step 1: Understand the skill with concrete trigger examples

This skill should trigger for prompts like:

- “Search this for me”
- “Look up the latest info on X”
- “What do sources say about Y?”
- “Find me current info on Z with links”
- “Use the synthesis proxy / SearXNG search”

The skill’s purpose is not generic product-research depth; it is a fast, cited open-web lookup through the existing synthesis proxy.

### Step 2: Plan the reusable contents

Use a small, deterministic resource set:

- `scripts/searxng_search.py` — the reusable API client + formatter
- `scripts/evals/test_proxy_healthz.sh` — QA connectivity check
- `scripts/evals/test_search_response.sh` — QA happy-path schema check
- `scripts/evals/test_error_handling.sh` — QA invalid query + bad-auth checks
- `scripts/evals/test_skill_structure.sh` — QA packaging/structure validation

Do **not** add extra docs unless they are genuinely needed. The API is simple enough that `SKILL.md` + one script are sufficient.

### Step 3: Initialize the skill

Use the mandated initializer:

```bash
python3 /app/skills/skill-creator/scripts/init_skill.py \
  searxng-search \
  --path /home/node/.openclaw/workspace/skills \
  --resources scripts
```

This yields:

```text
/home/node/.openclaw/workspace/skills/searxng-search/
├── SKILL.md
└── scripts/
```

Then replace the template content and add the eval scripts.

### Step 4: Implement the helper script

Create:

- `/home/node/.openclaw/workspace/skills/searxng-search/scripts/searxng_search.py`

Recommended CLI:

```bash
python3 skills/searxng-search/scripts/searxng_search.py healthz --json
python3 skills/searxng-search/scripts/searxng_search.py search --query 'golang context'
python3 skills/searxng-search/scripts/searxng_search.py search --query 'golang context' --json
```

Recommended functions/signatures:

```python
def require_credentials() -> tuple[str, str]
def build_basic_auth_header(username: str, password: str) -> str
def request_json(path: str, query: dict[str, str] | None = None, timeout: int = 30) -> tuple[int, dict | str | None]
def format_success(payload: dict) -> str
def format_error(status: int, payload: dict | str | None) -> str

def run_healthz(json_output: bool) -> int

def run_search(query: str, json_output: bool) -> int

def main() -> int
```

Implementation details:

- Use only Python stdlib: `argparse`, `base64`, `json`, `os`, `sys`, `urllib.parse`, `urllib.request`, `urllib.error`.
- Hardcode the default base URL as `https://synth.vandenborne.co`.
- Read credentials from `SYNTH_USERNAME` and `SYNTH_PASSWORD`.
- Send Basic Auth on **all** requests, including `/healthz`.
- Use `GET /v1/search?q=<urlencoded query>` exactly as specified.
- Exit `2` for local usage/configuration problems (missing env vars, empty query).
- Exit `1` for network/API/auth failures.
- Exit `0` for success.

#### Success formatting

Default stdout should be human-friendly, for example:

```text
Answer:
<answer>

Sources:
1. <title>
   <url>
   <snippet>
```

Formatting rules:

- Always show the synthesized answer first.
- Then show numbered sources with URL.
- Include snippet when present.
- Include engine only if useful; do not clutter the output.
- If no sources are returned, still show the answer cleanly.

#### Error handling rules

Map these cases to explicit user-facing messages:

- Missing env vars → `Missing SYNTH_USERNAME/SYNTH_PASSWORD`
- Blank query → `Query must not be empty`
- HTTP 401/403 → `Authentication failed for synth.vandenborne.co`
- Proxy JSON error shape → print `error.code` + `error.message`
- Non-JSON upstream error body → include HTTP status and raw text safely
- URL/network timeout → `Request failed: ...`

When `--json` is passed, print the parsed JSON payload on success. On errors, keep messages on stderr and preserve non-zero exit codes.

### Step 5: Write the skill file

Create/update:

- `/home/node/.openclaw/workspace/skills/searxng-search/SKILL.md`

Requirements:

- YAML frontmatter with `name` and `description` only
- concise body
- under 500 lines
- imperative wording

Recommended frontmatter:

```yaml
---
name: searxng-search
description: Use when the user wants a quick cited web search through the self-hosted synthesis proxy at synth.vandenborne.co, including prompts to search for current information, look something up online, or use the synthesis proxy/SearXNG instead of generic web search. Requires SYNTH_USERNAME and SYNTH_PASSWORD.
---
```

Recommended body outline:

1. Short overview
2. Required environment variables
3. Quick start commands
4. Workflow
   - validate creds
   - run the helper script via `exec`
   - summarize answer + sources for the user
   - surface clear auth/query/upstream errors
5. Notes
   - use `--json` when raw payload inspection is needed
   - prefer the bundled Python helper over ad-hoc shell/curl

### Step 6: Package/validate the skill

Run:

```bash
python3 /app/skills/skill-creator/scripts/package_skill.py \
  /home/node/.openclaw/workspace/skills/searxng-search \
  /home/node/.openclaw/workspace/skills/dist
```

This doubles as a structural validation step.

## QA evals and concrete test scripts

The QA agent should have explicit, headless commands to run.

### A. Proxy connectivity test

Create:

- `/home/node/.openclaw/workspace/skills/searxng-search/scripts/evals/test_proxy_healthz.sh`

Script behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
JSON="$(python3 "$SCRIPTS_DIR/searxng_search.py" healthz --json)"
python3 - "$JSON" <<'PY'
import json, sys
payload = json.loads(sys.argv[1])
assert payload.get("status") == "ok", payload
PY
```

Expected result: exit 0 and parsed payload contains `{"status":"ok"}`.

### B. Search response test

Create:

- `/home/node/.openclaw/workspace/skills/searxng-search/scripts/evals/test_search_response.sh`

Use a real query that should be stable enough for schema validation, e.g. `golang context package`.

Script behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
JSON="$(python3 "$SCRIPTS_DIR/searxng_search.py" search --query 'golang context package' --json)"
python3 - "$JSON" <<'PY'
import json, sys
payload = json.loads(sys.argv[1])
assert isinstance(payload.get("query"), str) and payload["query"].strip(), payload
assert isinstance(payload.get("answer"), str) and payload["answer"].strip(), payload
assert isinstance(payload.get("sources"), list), payload
assert isinstance(payload.get("meta"), dict), payload
meta = payload["meta"]
for key in ("model", "results_considered", "took_ms"):
    assert key in meta, payload
PY
```

Expected result: exit 0 and schema assertions pass.

### C. Skill structure test

Create:

- `/home/node/.openclaw/workspace/skills/searxng-search/scripts/evals/test_skill_structure.sh`

Script behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="/home/node/.openclaw/workspace/skills/searxng-search"
OUT_DIR="$(mktemp -d)"
python3 /app/skills/skill-creator/scripts/package_skill.py "$SKILL_DIR" "$OUT_DIR"
test -f "$OUT_DIR/searxng-search.skill"
```

Expected result: exit 0 and packaged `.skill` archive exists.

### D. Error handling test

Create:

- `/home/node/.openclaw/workspace/skills/searxng-search/scripts/evals/test_error_handling.sh`

Script behavior should cover both required failure modes.

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if python3 "$SCRIPTS_DIR/searxng_search.py" search --query '   ' >/tmp/searxng-invalid.out 2>/tmp/searxng-invalid.err; then
  echo 'expected blank-query failure' >&2
  exit 1
fi
grep -q 'Query must not be empty' /tmp/searxng-invalid.err

if SYNTH_USERNAME='bad-user' SYNTH_PASSWORD='bad-pass' \
  python3 "$SCRIPTS_DIR/searxng_search.py" search --query 'golang context package' \
  >/tmp/searxng-auth.out 2>/tmp/searxng-auth.err; then
  echo 'expected auth failure' >&2
  exit 1
fi
grep -Eq 'Authentication failed|HTTP 401|HTTP 403' /tmp/searxng-auth.err
```

Expected result: exit 0 only when both subtests fail in the expected, user-friendly way.

### E. JSON fix tests

Create in the repo:

- `/home/node/.openclaw/workspace/searchxng-synthesis-proxy/scripts/qa/test_json_fix.sh`

Script behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /home/node/.openclaw/workspace/searchxng-synthesis-proxy
go test ./internal/proxy -run 'Test(StripFences|ExtractFirstJSONObject|ParseSynthesisJSON)' -count=1 -v
go test ./...
```

Expected result: targeted prompt-parser tests pass and the full suite stays green.

## Suggested implementation order

1. Add `prompt_test.go` with failing tests first.
2. Refactor `stripFences`, add `extractFirstJSONObject`, and update `parseSynthesisJSON`.
3. Tighten `systemPrompt` wording.
4. Run `go test ./internal/proxy` and then `go test ./...`.
5. Initialize `skills/searxng-search` with `init_skill.py`.
6. Implement `scripts/searxng_search.py`.
7. Write the concise `SKILL.md`.
8. Add the four skill eval scripts.
9. Run the skill evals with valid credentials present.
10. Package the skill with `package_skill.py`.

## Final acceptance criteria

The combined work is complete when all of the following are true:

- The proxy no longer fails on common fenced/prose-wrapped JSON responses from the LLM.
- Prompt parsing coverage exists in focused unit tests rather than being buried only in `service_test.go`.
- `go test ./...` passes in the repo.
- `skills/searxng-search/SKILL.md` has valid frontmatter and stays concise.
- The skill uses `exec` + a deterministic Python helper to call the proxy with Basic Auth.
- The skill presents answer + sources cleanly.
- The skill reports missing credentials, bad auth, blank queries, and upstream errors clearly.
- QA has concrete scripts for healthz, search schema, skill structure, error handling, and JSON parser regression checks.
