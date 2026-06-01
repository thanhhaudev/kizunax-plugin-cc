---
description: Initialize Kizunax config (provider, model, API key) and build binary
argument-hint: '[--check] [--rebuild]'
disable-model-invocation: true
allowed-tools: Bash(go:*), Bash(/Users/*), Read
---

Run the Kizunax setup flow.

Steps:

1. If `${CLAUDE_PLUGIN_ROOT}/bin/kizunax` does not exist OR `--rebuild` is in `$ARGUMENTS`:
   - Run `bash ${CLAUDE_PLUGIN_ROOT}/../../scripts/build.sh` to build the binary.
   - If `go` is not in PATH, stop and tell the user to install Go ≥ 1.21.

2. Run:
   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup $ARGUMENTS
   ```

3. Return the stdout verbatim. Do not paraphrase.
