---
description: Show the stored final output for a finished Kizunax job in this repository
argument-hint: '[job-id-or-prefix]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" result $ARGUMENTS`
