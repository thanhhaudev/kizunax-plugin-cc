---
description: Adversarial code review challenging design choices, attack surface, and failure modes
argument-hint: '[--wait|--background] [--working-tree | --base <ref> | --commit <sha> | --from <sha> --to <sha>] [--paths a,b/] [--focus TEXT] [--quiet] [--verbose] [focus text]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read, AskUserQuestion
---

Run a Kizunax adversarial review that challenges the implementation approach and design choices.

Raw slash-command arguments:
`$ARGUMENTS`

Core constraint:
- This command is review-only. Do not fix issues, apply patches, or suggest you are about to make changes.
- Your only job is to run the review and return the binary's output verbatim. Do not paraphrase or summarize.

Execution mode:

### Base ref selection (run FIRST when no target flag is set)
- If the raw arguments already include any of `--working-tree`, `--base <ref>`, `--commit <sha>`, or `--from <a> --to <b>`, skip this step — the user has chosen.
- Otherwise, the user almost certainly wants a PR-style branch diff. Do NOT default to master/main; that includes everything on the integration branch.
- First try the current branch's upstream tracking ref via `git rev-parse --abbrev-ref @{upstream}`. If it resolves, use it as `--base <ref>` and tell the user `Base auto-resolved to <ref> (tracked upstream).`
- Otherwise, ask the user once via `AskUserQuestion`:
  - Header: `PR base branch`
  - Question: `This branch has no upstream set. Which branch does this PR merge into?`
  - Options (Recommended first): `develop`, `main`, `master`, plus one slot for `Working tree (uncommitted only)` if appropriate.
- Add `--base <chosen>` (or `--working-tree`) to the args before continuing.

### Size estimate
- Working-tree (default): inspect `git status --short --untracked-files=all`, `git diff --shortstat --cached`, `git diff --shortstat`.
- Branch (`--base <ref>`): `git diff --shortstat <ref>...HEAD`. If `<ref>` doesn't resolve (`git rev-parse --verify <ref>` fails), substitute with the repo's default branch from `git symbolic-ref --short refs/remotes/origin/HEAD` and rewrite the `--base` flag in the args to match; tell the user the substitution in one sentence. If even that fails (no remote), ask the user which ref to use.
- Commit (`--commit <sha>`): `git show --shortstat <sha>`.
- Range (`--from <a> --to <b>`): `git diff --shortstat <a>..<b>`.
- Treat untracked files as reviewable.
- Only conclude "nothing to review" when the relevant scope is genuinely empty.

### Scope guard (run AFTER size estimate, BEFORE Step 1)
- Skip this step if `--paths` is already in the raw arguments — user already scoped.
- If the estimated diff is **> 100 files OR > 150 KB**, the review will be truncated at the 256 KB bundle cap and a single LLM call may take 5-10+ minutes.
  - Ask once via `AskUserQuestion` with three options, fan-out labeled `(Recommended)` first:
    - Option 1 (recommended): `Fan out parallel` — split changed files by top-level dir, run N adversarial reviews in parallel (one per bucket), merge findings. Full coverage; wall time ≈ slowest bucket (~2-3 min). See "Fan-out flow" below.
    - Option 2: `Narrow with --paths` — abort and tell the user one sentence like "Re-run with `--paths <dir1,dir2,...>` to focus on specific dirs. Example: `--paths app/Http,app/Services`."
    - Option 3: `Continue full diff (slow)` — proceed but warn the user "Continuing with truncated bundle; review may take 5-10+ minutes and skip files past the 256 KB cap." then continue to Step 1.
- If diff is within limits, skip the question and continue to Step 1.
- If user chose `Fan out parallel`, jump to "Fan-out flow" below instead of Steps 1-2.
- If user chose `Continue full diff`, continue to Step 1.

### Fan-out flow (only when user picked "Fan out parallel")

1. **List changed files.** Pick the command matching the target:
   - Working-tree: `git diff --name-only HEAD` plus `git ls-files --others --exclude-standard`.
   - Branch (`--base <ref>`): `git diff --name-only <ref>...HEAD`.
   - Commit (`--commit <sha>`): `git diff-tree --no-commit-id --name-only -r <sha>`.
   - Range (`--from <a> --to <b>`): `git diff --name-only <a>..<b>`.

2. **Bucket by top-level dir.** Group files by their first path segment (`app/`, `resources/`, `database/`, `tests/`, `config/`, ...). Then balance:
   - If a single bucket has > 50 files, sub-group it by 2nd segment (e.g., split `app/` into `app/Http`, `app/Services`, `app/Models`).
   - If total bucket count > 10, merge the smallest buckets into a single "misc" bucket until count ≤ 10.
   - Drop empty buckets. If you end up with only 1 bucket, fan-out isn't useful — fall back to a normal single review (continue to Step 1).

3. **Pick provider once for ALL buckets.** Run Step 1 (provider routing) using the FULL diff size (not per-bucket), then skip Step 2 — fan-out is always foreground/wait so we can merge in the same turn.

4. **Spawn buckets in batches of max 4 (HEAT BOUND).** Do NOT spawn all N at once — on M1-class hardware, 9 concurrent kizunax processes peak above the laptop's thermal envelope. Run buckets in batches of `min(4, N)`:
   - Batch 1: buckets 1-4 → `Bash({run_in_background:true})` for each, wait for ALL four to complete (via `BashOutput`) before starting batch 2.
   - Batch 2: buckets 5-8 → same.
   - Final batch: remaining buckets (may be < 4).
   - Total batches = `ceil(N / 4)`. Total wall time ≈ `ceil(N/4) × per-bucket time`, still 4× faster than serial, but the laptop stays cool.

   Each bucket command uses `--paths <bucket-prefix>`, `--quiet`, and `--no-expand`:

   ```typescript
   Bash({
     command: `"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." adversarial-review --provider <chosen> <target-flags> --paths <bucket-prefix> --quiet --no-expand`,
     description: `Kizunax fan-out batch <b>/<B> bucket <i>/<N>: <bucket-prefix>`,
     run_in_background: true
   })
   ```

   The `--no-expand` flag is critical: it skips the v0.12 workspace symbol enrichment, which on monorepos (Laravel, Django, etc.) can spin each binary at 100% CPU for 10+ minutes without ever reaching the LLM call. Note that v0.19.0+ binaries auto-skip enrichment for workspaces > 3000 tracked files, but pass `--no-expand` explicitly so the prose still works on older binaries.

   Tell the user one line per batch: "Fan-out batch <b>/<B>: spawning <K> buckets in parallel (bucket-a, bucket-b, ...). Pre-cooling the rest."

5. **Poll until the batch completes, then move on.** Within each batch, use `BashOutput` on each shell ID. After every check, emit one progress line: "Fan-out: X/N buckets done across all batches." Cap the per-bucket wait at 15 minutes; if a bucket is still running past that, mark it skipped and continue with the next batch.

6. **Collect and merge findings.** For each completed bucket's stdout, parse the rendered adversarial-review markdown — specifically the findings table — and note the bucket source. Then render ONE unified report:
   - **TL;DR**: total findings count, breakdown by severity, list of buckets reviewed (and any skipped).
   - **Findings table** (all buckets merged): dedupe rows that share file path + line + title. Sort by severity (critical → important → minor), then by file path. Add a `Bucket` column or annotation showing which prefix surfaced the finding.
   - **Skipped buckets** (if any): list bucket + reason (timeout, API error, parse error).

7. **Output rules for fan-out report**:
   - Do NOT paraphrase individual findings — preserve the binary's exact wording.
   - Do NOT add a usage footer (each bucket used `--quiet`; fan-out report is the unified view).
   - End with one summary line: "Fan-out adversarial review complete. Reviewed N/N buckets, found X findings."

### Step 1 — Provider routing (ask FIRST, before wait/background)
- If the raw arguments already include `--provider openai` or `--provider anthropic`, skip this step entirely — the user's explicit choice wins.
- Otherwise, decide the recommendation based on estimated diff size + mode:
  - `< 15KB` AND standard review → recommend `--provider openai` (fast, ~20s typical)
  - `< 15KB` AND adversarial review → recommend `--provider anthropic` (heavier prompt; openai cliff lower for adversarial mode)
  - `≥ 15KB` (any mode) → recommend `--provider anthropic` (stable above the openai cliff observed at ~20-25KB in threshold test 2026-06-02)
- Ask once via `AskUserQuestion` with the recommended option labeled `(Recommended)` first:
  - Option 1 (recommended): the recommended provider with a one-line rationale
  - Option 2: the other provider with a one-line counter-rationale
- Remember the chosen provider for use in the Foreground / Background flow below.

### Step 2 — Wait vs background (ask SECOND, after provider is settled)
- If the raw arguments include `--wait`, run in the foreground (no question).
- If the raw arguments include `--background`, run in a Claude background task (no question).
- Otherwise, decide the recommendation from the size estimate:
  - Recommend wait only when the review is clearly tiny (1-2 files, no broader directory-sized change).
  - Otherwise (including unclear size), recommend background.
- Use `AskUserQuestion` exactly once with two options, recommended option first and suffixed `(Recommended)`:
  - `Wait for results`
  - `Run in background`

Foreground flow:
- If the raw arguments already include `--provider <name>`, run the binary without inserting a second flag:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." adversarial-review $ARGUMENTS
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." adversarial-review --provider <chosen> $ARGUMENTS
  ```
- Return the command stdout verbatim, exactly as-is.

Background flow:
- If the raw arguments already include `--provider <name>`, launch without inserting a second flag:
  ```typescript
  Bash({
    command: `"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." adversarial-review $ARGUMENTS`,
    description: "Kizunax adversarial review",
    run_in_background: true
  })
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```typescript
  Bash({
    command: `"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." adversarial-review --provider <chosen> $ARGUMENTS`,
    description: "Kizunax adversarial review",
    run_in_background: true
  })
  ```
- Do not call `BashOutput` or wait in this turn.
- After launching, tell the user: "Kizunax adversarial review started in the background. Claude will pick up the output automatically when it finishes; you can also check `/kizunax:status` for progress."

Argument handling:
- Preserve the user's arguments exactly, including any trailing free-form focus text.
- Do not strip `--wait`, `--background`, `--quiet`, or `--verbose` yourself.
- The binary parses `--background` as a synonym of foreground (no internal detach). Claude Code's `Bash(..., run_in_background:true)` is what actually detaches.
- If `--provider <name>` is already in the raw arguments, pass `$ARGUMENTS` unchanged.
- Otherwise, prepend `--provider <chosen-from-routing>` so the binary uses the routed provider.

Target flags (pick at most one; default `--working-tree`):
- `--working-tree` — Review uncommitted changes
- `--base <ref>` — Branch diff vs `<ref>`, e.g. `--base main`
- `--commit <sha>` — Single commit
- `--from <sha> --to <sha>` — Commit range

Combinable:
- `--paths a.go,subdir/` — Comma-separated path filter
- `--focus "text"` — Optional focus hint (or append free-form focus text after the flags)
- `--quiet` — Suppress trailing usage warning footer (for pipe / CI)
- `--verbose` — Log timing + model name to stderr

Adversarial mode emphasizes:
- Attack surface (injection, auth bypass, traversal)
- Concurrency and race conditions
- Failure modes (nil, empty, malformed, very-large inputs)
- Rollback safety, observability
- Resource leaks, missing timeouts

## Behavior (v0.11+)

- **Glossary auto-inject**: if the workspace contains `.kizunax/glossary.md`, `docs/glossary.md`, or `GLOSSARY.md` (priority in that order), its verbatim content (capped at 16 KiB) is prepended to the system prompt so the reviewer understands project-specific terminology. No file → silently skipped.
- **TL;DR summary**: a short executive summary is rendered at the top of the output when the review surfaces **3 or more findings**. Force on with `--summary`, force off with `--no-summary` (flags are mutually exclusive). The TL;DR replaces the model's own verbose summary in the same visual slot.

## Behavior (v0.12+)

- **Pre-flight context enrichment**: before sending the review prompt, kizunax
  scans the diff for symbol references (function calls, type usage, imports),
  walks the workspace for their definitions, and includes the relevant referenced
  files as a separate `## Referenced files for context (read-only)` section in
  the prompt. This gives the reviewer enough context to verify how diff'd code
  actually uses helpers, types, and constants — reducing speculative findings
  about imports/APIs the LLM cannot otherwise see.
- Up to 5 referenced files per symbol, up to 8 KiB per file excerpt, with the
  256 KiB total prompt cap unchanged. When the cap is hit, lowest-priority
  files (fewest symbol matches, then largest size) are dropped first; main
  review always proceeds.
- Multi-language: Go uses stdlib `go/ast` (precise). Other supported languages
  (TypeScript, JavaScript, Python, Rust, Java, C#, Ruby, PHP, Kotlin, Swift,
  Scala, C++, C) use universal regex by default, with WASM grammar precision
  arriving incrementally in v0.12.x follow-up patches.
- Lite build (`make build-lite` or `go build -tags lite`) skips WASM grammar
  embedding for a smaller binary; uses regex extraction across all languages.