---
description: Show active and recent Kizunax jobs for this repository
argument-hint: '[job-id-or-prefix] [--all]'
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run /kizunax:setup to build it."; exit 1; fi; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" status $ARGUMENTS`
