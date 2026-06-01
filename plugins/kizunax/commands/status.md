---
description: List Kizunax review jobs (or detail one) in this workspace
argument-hint: '[<job-id>]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Read
---

Show Kizunax background job status.

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, instruct the user to run `/kizunax:setup` first.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax status $ARGUMENTS
   ```

3. Return the command stdout verbatim.

No args → list all jobs in this workspace (newest first).
With `<job-id>` → render detail for that job (kind, status, timing, tokens, log path).
