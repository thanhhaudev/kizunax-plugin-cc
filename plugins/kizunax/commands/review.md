---
description: Review code changes via Kizunax (working tree, branch diff, commit, or paths)
argument-hint: '[--working-tree | --base <ref> | --commit <sha> | --from <sha> --to <sha>] [--paths a,b/] [--focus TEXT] [--verbose]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read
---

Run a Kizunax standard code review.

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, instruct the user to run `/kizunax:setup` first.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax review $ARGUMENTS
   ```

3. Return the command stdout verbatim. Do not paraphrase, summarize, or add commentary.

4. Do not fix any issues mentioned in the review output. The plugin is read-only.

Target flags (pick at most one; default `--working-tree`):
- `--working-tree` — Review uncommitted changes (default)
- `--base <ref>` — Review branch diff vs `<ref>`, e.g. `--base main`
- `--commit <sha>` — Review a single commit
- `--from <sha> --to <sha>` — Review a commit range

Combinable with any target:
- `--paths a.go,subdir/` — Comma-separated path filter
- `--focus "text"` — Optional focus hint
