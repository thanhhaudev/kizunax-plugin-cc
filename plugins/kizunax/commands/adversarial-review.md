---
description: Adversarial code review (skeptic stance, attack surface, failure modes)
argument-hint: '[target-flags] [--paths a,b/] [--focus TEXT] [--verbose]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Bash(git:*), Read
---

Run a Kizunax adversarial review (security-minded, skeptic stance, attack surface focus).

Steps:

1. Verify `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` exists. If not, instruct the user to run `/kizunax:setup` first.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax adversarial-review $ARGUMENTS
   ```

3. Return the command stdout verbatim. Do not paraphrase, summarize, or add commentary.

4. Do not fix any issues mentioned in the review output. The plugin is read-only.

Same target flags as `/kizunax:review`. Adversarial mode focuses on:
- Attack surface (injection, auth bypass, traversal)
- Concurrency & race conditions
- Failure modes (nil/empty/malformed/very-large inputs)
- Rollback safety, observability
- Resource leaks, missing timeouts

Use `--focus "auth flow"` (or similar) to direct attention to a specific concern.
