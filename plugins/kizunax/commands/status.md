---
description: Show active and recent Kizunax jobs for this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." status $ARGUMENTS`

Render the binary's stdout verbatim. Do not paraphrase, summarize, or reformat. Preserve every column of the Markdown table including the Actions column literals (`/kizunax:result <id>`, `/kizunax:cancel <id>`).
