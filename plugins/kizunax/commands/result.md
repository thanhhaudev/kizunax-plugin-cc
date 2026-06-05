---
description: Show the stored final output for a finished Kizunax job in this repository
argument-hint: '[job-id-or-prefix]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." result $ARGUMENTS`
