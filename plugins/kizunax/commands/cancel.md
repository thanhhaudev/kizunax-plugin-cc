---
description: Cancel an active background Kizunax job in this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

Pre-flight:
- Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, tell the user: "Binary missing — run `/kizunax:setup` first to build it." Then stop.

Run:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/kizunax cancel $ARGUMENTS
```

Output rules:
- Return the binary's output verbatim. Do not summarize.
- No argument: cancel the only active job in the current session if exactly one exists.
- A job id or unique prefix: cancel that specific job.
- `--all` bypasses the session filter (search active jobs across all sessions).
- Ambiguous prefix or no match: error rendered as-is.
