---
description: Cancel an active background Kizunax job in this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`[ -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ] || { echo "Binary missing — run /kizunax:setup to build it."; exit 1; }; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" cancel $ARGUMENTS`
