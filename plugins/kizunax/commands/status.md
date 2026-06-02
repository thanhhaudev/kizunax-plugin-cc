---
description: Show active and recent Kizunax jobs for this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

Pre-flight:
- Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, tell the user: "Binary missing — run `/kizunax:setup` first to build it." Then stop.

Run:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/kizunax status $ARGUMENTS
```

Output rules:
- Return the binary's output verbatim. Do not summarize or condense.
- If no argument: a Markdown table for the current session is rendered; preserve every column including the Actions column (which embeds `/kizunax:result <id>` and `/kizunax:cancel <id>` literals).
- If a job id or unique prefix is passed: full job detail is rendered; pass through as-is.
- `--all` bypasses the session filter and lists every job in the workspace.
- Ambiguous id prefixes return an error suggesting a longer prefix; render that verbatim too.
