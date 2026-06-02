---
description: Show per-key quota usage (Coding Plan + Public v1 credits)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

Pre-flight:
- Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, tell the user: "Binary missing — run `/kizunax:setup` first to build it." Then stop.

Run:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/kizunax" usage $ARGUMENTS
```

Output rules:

- Return the binary's output verbatim. Do not summarize or condense.
- The table shows: masked key, Coding Plan quota (request count + reset window), Public v1 credits (token count + monthly cycle).
- A `> ⚠ Usage warning` line is appended below the table for any key whose remaining quota is low.
- Do not call any further tools after the Bash invocation.
