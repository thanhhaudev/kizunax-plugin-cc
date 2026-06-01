---
description: Render the result of a finished Kizunax review job
argument-hint: '<job-id>'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Read
---

Show the full review output (findings, recommendations, next steps) of a completed Kizunax job.

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax result $ARGUMENTS
   ```

3. Return the command stdout verbatim. Do not paraphrase, fix, or summarize.

If the job is still running → output asks you to check `/kizunax:status` again later.
If the job failed → output prints the error reason and log path.
