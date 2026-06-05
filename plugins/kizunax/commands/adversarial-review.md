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

### Size estimate
- Working-tree (default): inspect `git status --short --untracked-files=all`, `git diff --shortstat --cached`, `git diff --shortstat`.
- Branch (`--base <ref>`): `git diff --shortstat <ref>...HEAD`.
- Commit (`--commit <sha>`): `git show --shortstat <sha>`.
- Range (`--from <a> --to <b>`): `git diff --shortstat <a>..<b>`.
- Treat untracked files as reviewable.
- Only conclude "nothing to review" when the relevant scope is genuinely empty.

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
  if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi
  ${CLAUDE_PLUGIN_ROOT}/bin/kizunax adversarial-review $ARGUMENTS
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```bash
  if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi
  ${CLAUDE_PLUGIN_ROOT}/bin/kizunax adversarial-review --provider <chosen> $ARGUMENTS
  ```
- Return the command stdout verbatim, exactly as-is.

Background flow:
- If the raw arguments already include `--provider <name>`, launch without inserting a second flag:
  ```typescript
  Bash({
    command: `if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi; ${CLAUDE_PLUGIN_ROOT}/bin/kizunax adversarial-review $ARGUMENTS`,
    description: "Kizunax adversarial review",
    run_in_background: true
  })
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```typescript
  Bash({
    command: `if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi; ${CLAUDE_PLUGIN_ROOT}/bin/kizunax adversarial-review --provider <chosen> $ARGUMENTS`,
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