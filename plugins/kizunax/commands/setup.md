---
description: Configure Kizunax via a local web form (browser)
argument-hint: ''
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*)
---

Configure Kizunax in a local browser form. The CLI starts a server on 127.0.0.1, opens your browser to a one-shot URL, then runs in the background until you save the form (or 5 minutes pass).

Run:

```bash
if [ ! -f "${CLAUDE_PLUGIN_ROOT}/bin/kizunax" ]; then
  echo "Binary missing — run ./install.sh at the repo root."
  exit 1
fi
"${CLAUDE_PLUGIN_ROOT}/bin/kizunax" setup --web
```

Show the binary's stdout verbatim. It prints two lines — a label and the localhost URL. The browser should open automatically; if it does not, click or copy the URL.

Output rules:

- Print the binary's stdout verbatim. Do not paraphrase.
- Do not call any further tools after the Bash invocation.
- If the user reports the worker exited unexpectedly, suggest re-running `/kizunax:setup`.
