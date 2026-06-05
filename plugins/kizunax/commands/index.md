---
description: Manage Kizunax workspace AST index (status/sync/enable/disable/toggle/info/purge)
argument-hint: '<subcommand> [args]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

Manage the v0.13 workspace AST index. Subcommands:

- `status` — show the current flag state, index path, file count, symbol counts.
- `enable` — persist `KIZUNAX_USE_INDEX=1` for this workspace (survives sessions).
- `disable` — turn the v2 resolver off for this workspace (revert to v1 regex).
- `toggle` — flip the current state.
- `sync` — force a full rebuild of the index now (slow on first run; ~2 min for 200 files).
- `info <symbol>` — dump all index entries for a given symbol name.
- `purge` — delete the on-disk index directory.

Run the binary with the arguments the user provided. Default to `status` if no subcommand was passed.

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run ./install.sh at the repo root." index $ARGUMENTS
```

Show the binary's stdout verbatim. Do not paraphrase. Do not call any further tools after the Bash invocation.
