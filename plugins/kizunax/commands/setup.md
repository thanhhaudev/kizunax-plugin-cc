---
description: Configure Kizunax via a local web form (browser)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then echo "Binary missing — run ./install.sh at the repo root."; exit 1; fi; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" setup --web`
