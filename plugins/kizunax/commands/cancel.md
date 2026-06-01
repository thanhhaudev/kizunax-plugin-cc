---
description: Cancel a running Kizunax review job (SIGTERM the worker)
argument-hint: '<job-id>'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*)
---

Stop a running Kizunax background job.

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax cancel $ARGUMENTS
   ```

3. Return the command stdout verbatim.

The worker process tree receives SIGTERM and the job record is marked `cancelled`. Already-finished jobs cannot be cancelled (the command will say so).
