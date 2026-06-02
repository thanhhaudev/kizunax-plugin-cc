---
description: Configure Kizunax via a local web form (browser)
argument-hint: ''
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*), Bash(test:*)
---

Configure Kizunax in a local browser form. No data is sent over the network — the form is served from `127.0.0.1` on a one-shot random token.

Steps you MUST follow in order:

1. Verify the binary exists. Run:

   ```bash
   test -f ${CLAUDE_PLUGIN_ROOT}/bin/kizunax && echo "OK" || echo "MISSING"
   ```

   If the result is `MISSING`, tell the user to run the installer at the repo root (`./install.sh` on macOS/Linux, `.\install.ps1` on Windows) and stop.

2. Start the web setup. Run:

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --web
   ```

3. Show the binary's stdout verbatim — it prints one line with a localhost URL. Tell the user to open that URL in their browser, fill the form, and click Save. The CLI exits automatically once they save (or after 5 minutes of inactivity).

Output rules:

- Print the binary's stdout verbatim. Do not paraphrase.
- Do not call any further tools.
- If the user reports the CLI exited with a timeout or cancel, suggest re-running `/kizunax:setup`.
