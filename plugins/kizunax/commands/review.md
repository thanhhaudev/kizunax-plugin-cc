---
description: Review code changes via Kizunax (working tree, branch diff, commit, or paths)
argument-hint: '[--working-tree | --base <ref> | --commit <sha> | --from <sha> --to <sha>] [--strategy auto|single|fanout] [--paths a,b/] [--provider openai|anthropic] [--model <name>] [--focus TEXT] [--quiet] [--verbose]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read, AskUserQuestion
---

Run a Kizunax standard code review.

Raw slash-command arguments:
`$ARGUMENTS`

## Behavior (v0.26.0+)

The binary handles all orchestration internally — base ref auto-resolution
(v0.20), smart working-tree default (v0.21), diff size estimation, and
parallel fan-out for large diffs (v0.26). The slash command only adds
intent-level decisions on top.

## Steps

### Step 1 — Mode (ask only if `--strategy` is not in args)

If the raw arguments include `--strategy <auto|single|fanout>`, skip this step.
Otherwise ask once via `AskUserQuestion`:
- Header: `Review mode`
- Question: `How thorough should this review be?`
- Options (Recommended first):
  - `Auto (Recommended)` — Binary decides: fan-out for big diffs, single for small. Provider auto-routes.
  - `Quick (single, fast)` — One review, fast provider. Good for tiny diffs.
  - `Thorough (fan-out, deep)` — Force parallel buckets + deep provider. Good for large feature branches.

Map the answer to a flag and append to args:
- Auto → `--strategy=auto`
- Quick → `--strategy=single`
- Thorough → `--strategy=fanout`

### Step 1.5 — Inline context (ONLY if `--add-context-prompt` is in args)

If `--add-context-prompt` is NOT in the raw arguments, skip this step entirely.

Otherwise ask once via `AskUserQuestion`:
- Header: `Inline context`
- Question: `Any context to add for THIS REVIEW only?`
- Options:
  - `Skip` — no inline context; proceed to Step 2
  - `Type now` — capture free-form text from the next user message,
    base64-encode it, and append `--context-text "<encoded>"` to the args
    before continuing to Step 2

This is per-review only — it is NOT persisted to .kizunax/review-context.md.
For persistent context use `/kizunax:context` instead.

### Step 2 — Invoke the binary

Pick execution mode based on the effective `--strategy`:
- `--strategy=fanout` → run in BACKGROUND (large diff, 5–40 min wall time, do not block the conversation)
- `--strategy=auto` or `--strategy=single` → run in FOREGROUND (small/medium diff, typically 30 s – 3 min)

Foreground:
```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." review $ARGUMENTS
```

Background:
```typescript
Bash({
  command: `"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." review $ARGUMENTS`,
  description: "Kizunax review (fan-out)",
  run_in_background: true
})
```
After spawning in background, tell the user one line:
"Kizunax fan-out review started in the background. Claude will pick up the
output automatically when it finishes; you can also check `/kizunax:status`
for progress."

Return the binary's stdout VERBATIM (foreground) or the background-task
notification line (background). Do not paraphrase, do not summarize, do not
add commentary above or below.

## Core constraint
- This command is review-only. Do not fix issues, apply patches, or suggest
  you are about to make changes. Your only job is to run the binary and
  return its output verbatim.

## Flag reference

Target (pick at most one; binary defaults to working-tree, then auto-flips to
PR diff if working tree is clean):
- `--working-tree` — Review uncommitted changes
- `--base <ref>` — Branch diff vs `<ref>` (use `--base auto` for upstream detection)
- `--commit <sha>` — Single commit
- `--from <sha> --to <sha>` — Commit range

Combinable:
- `--strategy <auto|single|fanout>` — Override mode (default `auto`)
- `--paths a.go,subdir/` — Comma-separated path filter
- `--provider <openai|anthropic>` — Override provider routing
- `--model <name>` — Override model name (e.g. `--model claude-opus-4-7`)
- `--focus "text"` — Optional focus hint
- `--quiet` — Suppress trailing usage warning footer
- `--verbose` — Log timing + model name + fan-out telemetry to stderr
