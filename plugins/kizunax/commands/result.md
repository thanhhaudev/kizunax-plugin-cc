---
description: Show the stored final output for a finished Kizunax job in this repository
argument-hint: '[job-id-or-prefix]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`[ -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ] || { echo "Binary missing — run /kizunax:setup to build it."; exit 1; }; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" result $ARGUMENTS`
