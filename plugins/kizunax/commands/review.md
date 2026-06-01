---
description: Review uncommitted changes (working-tree) using Kizunax
argument-hint: '[--verbose]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read
---

Run a Kizunax review on the current working tree.

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, instruct the user to run `/kizunax:setup` first.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax review --working-tree $ARGUMENTS
   ```

3. Return the command stdout verbatim. Do not paraphrase, summarize, or add commentary.

4. Do not fix any issues mentioned in the review output. The plugin is read-only.

v0.1 limitation: only working-tree review is supported. Branch diff / commit range / paths will land in later versions.
