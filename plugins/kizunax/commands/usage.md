---
description: Show per-key quota usage (Coding Plan + Public v1 credits)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`[ -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ] || { echo "Binary missing — run /kizunax:setup to build it."; exit 1; }; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" usage $ARGUMENTS`
