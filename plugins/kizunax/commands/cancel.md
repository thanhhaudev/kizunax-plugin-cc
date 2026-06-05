---
description: Cancel an active background Kizunax job in this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run /kizunax:setup to build it." cancel $ARGUMENTS`
