---
description: Manage Kizunax bundle expansion strategies (status/enable/disable/set/reset)
argument-hint: '<subcommand> [csv]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

Manage v1.1.0 bundle expansion strategies for KizunaX reviews. Persisted per-workspace at `~/.kizunax/state/<ws-hash>/expansion.json`.

Subcommands:

- `status` — print workspace path, env state, persisted strategies, and precedence.
- `enable <csv>` — additively turn on listed strategies (others stay as they are).
- `disable <csv>` — additively turn off listed strategies.
- `set <csv>` — replace state with exactly the listed strategies (unlisted = off).
- `reset` — delete the state file; revert all strategies to default off.

Recognized CSV tokens: `callers`, `typedefs`, `tests`, `all`, `none` (case-insensitive).

Examples:
- `/kizunax:expansion status`
- `/kizunax:expansion enable callers,tests`
- `/kizunax:expansion disable typedefs`
- `/kizunax:expansion set callers,tests`
- `/kizunax:expansion reset`

Precedence (earliest wins): kill switch (`KIZUNAX_DISABLE_EXPAND=1`) > CLI flag on `kizunax review` (`--expand-callers/typedefs/tests/-all`, `--no-expand`) > one-shot env (`KIZUNAX_EXPAND=<csv>`) > state file > default off.

Run the binary with the arguments the user provided. Default to `status` if no subcommand was passed.

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run ./install.sh at the repo root." expansion $ARGUMENTS
```

Show the binary's stdout verbatim. Do not paraphrase. Do not call any further tools after the Bash invocation.
