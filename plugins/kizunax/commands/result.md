---
description: Show the stored final output for a finished Kizunax job in this repository
argument-hint: '[job-id-or-prefix]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." result $ARGUMENTS`

Render the binary's stdout verbatim. Do not paraphrase, summarize, or reformat. Preserve the full review markdown including findings table and any follow-up command literals.
