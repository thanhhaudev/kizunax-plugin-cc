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
- If the raw arguments include `--wait`, run in the foreground (no question).
- If the raw arguments include `--background`, run in a Claude background task (no question).
- Otherwise, estimate the review size before asking:
  - Working-tree (default): inspect `git status --short --untracked-files=all`, `git diff --shortstat --cached`, `git diff --shortstat`.
  - Branch (`--base <ref>`): `git diff --shortstat <ref>...HEAD`.
  - Commit (`--commit <sha>`): `git show --shortstat <sha>`.
  - Treat untracked files as reviewable.
  - Only conclude "nothing to review" when the relevant scope is genuinely empty.
  - Recommend wait only when the review is clearly tiny (1-2 files, no broader directory-sized change).
  - Otherwise (including unclear size), recommend background.
- Use `AskUserQuestion` exactly once with two options, recommended option first and suffixed `(Recommended)`:
  - `Wait for results`
  - `Run in background`

Foreground flow:
```bash
${CLAUDE_PLUGIN_ROOT}/bin/kizunax review $ARGUMENTS
```
- Return the command stdout verbatim, exactly as-is.

Background flow:
- Launch with `Bash` in the background:
```typescript
Bash({
  command: `${CLAUDE_PLUGIN_ROOT}/bin/kizunax review $ARGUMENTS`,
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
