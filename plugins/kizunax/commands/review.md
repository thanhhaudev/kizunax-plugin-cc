---
description: Review code changes via Kizunax (working tree, branch diff, commit, or paths)
argument-hint: '[--wait|--background] [--working-tree | --base <ref> | --commit <sha> | --from <sha> --to <sha>] [--paths a,b/] [--focus TEXT] [--quiet] [--verbose]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read, AskUserQuestion
---

Run a Kizunax standard code review.

Raw slash-command arguments:
`$ARGUMENTS`

Core constraint:
- This command is review-only. Do not fix issues, apply patches, or suggest you are about to make changes.
- Your only job is to run the review and return the binary's output verbatim. Do not paraphrase or summarize.

Pre-flight:
- Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, tell the user: "Binary missing — run `/kizunax:setup` first to build it." Then stop.

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
  ${CLAUDE_PLUGIN_ROOT}/bin/kizunax review $ARGUMENTS
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```bash
  ${CLAUDE_PLUGIN_ROOT}/bin/kizunax review --provider <chosen> $ARGUMENTS
  ```
- Return the command stdout verbatim, exactly as-is.

Background flow:
- If the raw arguments already include `--provider <name>`, launch without inserting a second flag:
  ```typescript
  Bash({
    command: `${CLAUDE_PLUGIN_ROOT}/bin/kizunax review $ARGUMENTS`,
    description: "Kizunax review",
    run_in_background: true
  })
  ```
- Otherwise, prepend the routed choice from Step 1:
  ```typescript
  Bash({
    command: `${CLAUDE_PLUGIN_ROOT}/bin/kizunax review --provider <chosen> $ARGUMENTS`,
    description: "Kizunax review",
    run_in_background: true
  })
  ```
- Do not call `BashOutput` or wait in this turn.
- After launching, tell the user: "Kizunax review started in the background. Claude will pick up the output automatically when it finishes; you can also check `/kizunax:status` for progress."

Argument handling:
- Preserve the user's arguments exactly.
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
- `--focus "text"` — Optional focus hint
- `--quiet` — Suppress trailing usage warning footer (for pipe / CI)
- `--verbose` — Log timing + model name to stderr

## Behavior (v0.11+)

- **Glossary auto-inject**: if the workspace contains `.kizunax/glossary.md`, `docs/glossary.md`, or `GLOSSARY.md` (priority in that order), its verbatim content (capped at 16 KiB) is prepended to the system prompt so the reviewer understands project-specific terminology. No file → silently skipped.
- **TL;DR summary**: a short executive summary is rendered at the top of the output when the review surfaces **3 or more findings**. Force on with `--summary`, force off with `--no-summary` (flags are mutually exclusive). The TL;DR replaces the model's own verbose summary in the same visual slot.
