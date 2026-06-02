---
description: Show the stored final output for a finished Kizunax job in this repository
argument-hint: '[job-id-or-prefix]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

Pre-flight:
- Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, tell the user: "Binary missing — run `/kizunax:setup` first to build it." Then stop.

Run:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/kizunax result $ARGUMENTS
```

Output rules:
- Return the binary's output verbatim. Do not summarize, condense, or reformat.
- No argument: shows the most recent job (any session).
- A job id or unique prefix: pass through. Ambiguous prefix → error; render as-is.
- Result spans ALL sessions (not just the current one) — users can inspect historical reviews.
