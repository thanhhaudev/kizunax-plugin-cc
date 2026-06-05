---
description: Configure Kizunax via a local web form (browser)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

!`[ -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ] || { echo "Binary missing — run ./install.sh at the repo root."; exit 1; }; "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" setup --web`
