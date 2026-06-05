---
description: Configure Kizunax via a local web form (browser)
argument-hint: ''
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/scripts/run.sh:*)
---

!`"${CLAUDE_PLUGIN_ROOT}/scripts/run.sh" "Binary missing — run ./install.sh at the repo root." setup --web`

Render the binary's stdout verbatim. It prints the localhost URL where the setup form is served — preserve it exactly so the user can click or copy.
