---
description: Configure Kizunax provider, model, and API key (works in Claude Code chat)
argument-hint: ''
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*), Bash(test:*), Bash(bash:*), AskUserQuestion
---

Configure Kizunax without leaving Claude Code. Uses `AskUserQuestion` for provider / base URL / model. The API key step is handled via a `!`-prefix command so the key never enters the model's context.

Steps you MUST follow in order:

1. Verify the binary exists. Run:

   ```bash
   test -f ${CLAUDE_PLUGIN_ROOT}/bin/kizunax && echo "OK" || echo "MISSING"
   ```

   If the result is `MISSING`, tell the user to run the installer at the repo root (`./install.sh` on macOS/Linux, `.\install.ps1` on Windows) and stop.

2. Read current config inventory. Run:

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --json
   ```

   Parse the JSON. Note `providers.openai.has_api_key` and `providers.anthropic.has_api_key` for the branch in step 6.

3. **AskUserQuestion #1** — Which provider to configure?

   - Question: `Which provider do you want to configure?`
   - Header: `Provider`
   - multiSelect: false
   - Options:
     - `OpenAI-compatible` — `KizunaX Coding Plan default; works with OpenAI, Groq, Together, OpenRouter, Ollama, vLLM, etc.`
     - `Anthropic-compatible` — `KizunaX Anthropic-compat endpoint or Anthropic direct.`

4. **AskUserQuestion #2** — Base URL.

   If provider chosen in step 3 is `OpenAI-compatible`:

   - Question: `Base URL for OpenAI-compatible endpoint?`
   - Header: `Base URL`
   - multiSelect: false
   - Options:
     - `https://kizunax.io/api/coding/v1` — `KizunaX Coding Plan default (Recommended)`
     - `https://api.openai.com/v1` — `OpenAI direct`

   If `Anthropic-compatible`:

   - Question: `Base URL for Anthropic-compatible endpoint?`
   - Header: `Base URL`
   - multiSelect: false
   - Options:
     - `https://kizunax.io/api/coding/anthropic/v1` — `KizunaX Anthropic-compat default (Recommended)`
     - `https://api.anthropic.com/v1` — `Anthropic direct`

5. **AskUserQuestion #3** — Model.

   If OpenAI-compatible:

   - Question: `Which model do you want to use?`
   - Header: `Model`
   - multiSelect: false
   - Options:
     - `coding/MiniMax-M2.7` — `KizunaX MiniMax default (Recommended)`
     - `coding/kimi-k2.6` — `Moonshot Kimi K2.6 (KizunaX exclusive)`

   If Anthropic-compatible:

   - Question: `Which model do you want to use?`
   - Header: `Model`
   - multiSelect: false
   - Options:
     - `MiniMax-M2.7-highspeed` — `KizunaX MiniMax-M2.7 (Recommended)`
     - `MiniMax-M2.5-highspeed` — `KizunaX MiniMax-M2.5`

6. **Branch on `has_api_key` for the chosen provider in the JSON from step 2**:

   - **If `has_api_key` is `true`**: ask AskUserQuestion #4:
     - Question: `An API key for this provider is already saved. What do you want to do?`
     - Header: `API key`
     - multiSelect: false
     - Options:
       - `Reuse existing key` — `Keep the saved key, only update base URL and model.`
       - `Replace with new key` — `Enter a new key via the prompt.`

     If the user picks `Reuse existing key`, run:

     ```bash
     ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --save --provider <PROVIDER> --base-url <BASE_URL> --model <MODEL> --reuse-api-key --set-default
     ```

     Substitute the chosen provider/URL/model. Show the binary's stdout verbatim. End of slash command.

   - **If `has_api_key` is `false` OR the user picked `Replace with new key`**: print exactly this block, then end the slash command. Do NOT call any further tools.

     ```
     To finish, paste the following into the prompt and replace YOUR_KEY with your real key:

     ! ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --save \
         --provider <PROVIDER> \
         --base-url <BASE_URL> \
         --model <MODEL> \
         --api-key YOUR_KEY \
         --set-default

     The key never enters the model's context — the ! prefix sends the command straight to the shell.
     ```

     Substitute `<PROVIDER>`, `<BASE_URL>`, `<MODEL>` literally — the user replaces only `YOUR_KEY`.

Output rules:

- For the Reuse branch: print the binary's stdout verbatim — do not paraphrase or summarize.
- For the Replace / fresh branch: print the `!` block exactly as shown above. Do not add commentary after it.
- Do not fix any issues mentioned by the user; the slash command is read-only on the codebase.
