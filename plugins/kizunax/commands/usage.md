---
description: Show per-key quota usage (Coding Plan + Public v1 credits)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." usage $ARGUMENTS`

Render the binary's stdout verbatim. Do not paraphrase or summarize. Preserve the per-key quota table (progress bars + percentages + reset windows) and any `> ⚠ Usage warning` line below it.
