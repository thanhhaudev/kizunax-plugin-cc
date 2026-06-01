---
description: Configure Kizunax provider(s), model, and API key (works in Claude Code chat)
argument-hint: ''
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/kizunax:*), Bash(test:*), Bash(bash:*), Bash(cat:*), Bash(chmod:*), Bash(mkdir:*), AskUserQuestion
---

Configure Kizunax inside Claude Code. Lets you set up one or both providers in a single run. The API key step is handled via a short `!`-prefix command where you paste only the key — the key never enters the model's context.

Steps you MUST follow in order:

1. Verify the binary exists. Run:

   ```bash
   test -f ${CLAUDE_PLUGIN_ROOT}/bin/kizunax && echo "OK" || echo "MISSING"
   ```

   If the result is `MISSING`, tell the user to run the installer at the repo root (`./install.sh` on macOS/Linux, `.\install.ps1` on Windows) and stop.

2. Read the current config inventory. Run:

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --json
   ```

   Parse the JSON. Note `providers.openai.has_api_key` and `providers.anthropic.has_api_key` for the strategy branch in step 6.

3. **AskUserQuestion #1 (multiSelect=true)** — Which providers to configure?

   - Question: `Which providers do you want to configure?`
   - Header: `Providers`
   - multiSelect: true
   - Options:
     - `OpenAI-compatible` — `KizunaX Coding Plan default; works with OpenAI, Groq, Together, OpenRouter, Ollama, vLLM, etc.`
     - `Anthropic-compatible` — `KizunaX Anthropic-compat endpoint or Anthropic direct.`

   The user picks 1 or 2. Use the selection order as configuration order (first selected becomes the default provider in the pending file).

4. For EACH selected provider in order, ask two questions.

   **For OpenAI-compatible:**

   - AskUserQuestion (base URL):
     - Question: `Base URL for OpenAI-compatible endpoint?`
     - Header: `OpenAI URL`
     - multiSelect: false
     - Options:
       - `https://kizunax.io/api/coding/v1` — `KizunaX Coding Plan default (Recommended)`
       - `https://api.openai.com/v1` — `OpenAI direct`
   - AskUserQuestion (model):
     - Question: `Which model do you want to use for OpenAI-compatible?`
     - Header: `OpenAI model`
     - multiSelect: false
     - Options:
       - `coding/MiniMax-M2.7` — `KizunaX MiniMax default (Recommended)`
       - `coding/kimi-k2.6` — `Moonshot Kimi K2.6 (KizunaX exclusive)`

   **For Anthropic-compatible:**

   - AskUserQuestion (base URL):
     - Question: `Base URL for Anthropic-compatible endpoint?`
     - Header: `Anthropic URL`
     - multiSelect: false
     - Options:
       - `https://kizunax.io/api/coding/anthropic/v1` — `KizunaX Anthropic-compat default (Recommended)`
       - `https://api.anthropic.com/v1` — `Anthropic direct`
   - AskUserQuestion (model):
     - Question: `Which model do you want to use for Anthropic-compatible?`
     - Header: `Anthropic model`
     - multiSelect: false
     - Options:
       - `MiniMax-M2.7-highspeed` — `KizunaX MiniMax-M2.7 (Recommended)`
       - `MiniMax-M2.5-highspeed` — `KizunaX MiniMax-M2.5`

5. Write the pending setup file. The shape MUST be exactly the JSON below — order in `providers` reflects selection order; `default_provider` is the first selected name.

   Run (replace `<DEFAULT>`, the two provider blocks, and the entries array with the chosen values; OMIT a provider block if the user did not select it):

   ```bash
   mkdir -p ~/.kizunax && chmod 700 ~/.kizunax
   cat > ~/.kizunax/.pending-setup.json <<'EOF'
   {
     "default_provider": "<DEFAULT>",
     "providers": [
       {"name": "openai",    "base_url": "<OPENAI_URL>",    "model": "<OPENAI_MODEL>"},
       {"name": "anthropic", "base_url": "<ANTHROPIC_URL>", "model": "<ANTHROPIC_MODEL>"}
     ]
   }
   EOF
   chmod 600 ~/.kizunax/.pending-setup.json
   ```

   Use exactly one entry if only one provider was selected. After this write, the pending file is on disk.

6. Decide the API-key strategy. Look at the `--json` output from step 2:

   - Let `reusable` be the list of SELECTED providers whose `has_api_key` was `true` (i.e., a key for that provider already exists in `~/.kizunax/config.json`).

   - **Case A** — All selected providers are in `reusable`:
     - AskUserQuestion:
       - Question: `Existing API keys are saved for the chosen provider(s). Reuse them?`
       - Header: `API key`
       - multiSelect: false
       - Options:
         - `Reuse existing key(s)` — `Keep the saved keys; only update base URL and model.`
         - `Replace with new key(s)` — `Enter new keys via the prompt.`
     - If `Reuse`, run:

       ```bash
       ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --apply --reuse
       ```

       Show the binary's stdout verbatim. End of slash command.

   - **Case B** — User picked exactly 1 provider, and `reusable` is empty:
     - Print the paste block from step 7 (Variant 1) with the 1 provider's flag. Skip the next question.

   - **Case C** — User picked 2 providers and at least one needs a new key (i.e., not all are reusable):
     - AskUserQuestion:
       - Question: `Same API key for both providers, or different per provider?`
       - Header: `Key strategy`
       - multiSelect: false
       - Options (only include `Reuse where possible` if `reusable` is non-empty):
         - `Same key for both` — `One key applies to openai and anthropic.`
         - `Different key per provider` — `Enter each key separately.`
         - `Reuse where possible, paste new for the rest` — `For providers with an existing key, keep it; for the others, enter a new key.`
     - On `Reuse where possible`, run:

       ```bash
       ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --apply --reuse
       ```

       This applies existing keys to any selected providers that have one. If any pending providers remain (no existing key), the binary will report "Still pending: ...". Continue to print one paste line per remaining provider as in Variant 2.

7. Print the paste block(s) and END the slash command. Do not invoke any further tools after printing.

   **Variant 1 — Same key for all selected providers** (Case B, or Case C with `Same key`):

   ```
   Paste your API key into the prompt (just the key, then send):

   ! ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --apply --key 
   ```

   Note the trailing space after `--key`. The user appends the key and sends.

   **Variant 2 — Different keys per provider** (Case C with `Different key`, or remainder of `Reuse where possible`):

   ```
   Paste each provider's key separately (run each in sequence; just the key, then send):

   ! ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --apply --openai-key 
   ! ${CLAUDE_PLUGIN_ROOT}/bin/kizunax setup --apply --anthropic-key 
   ```

   If only ONE provider remains pending after a `Reuse where possible` partial-apply, print only the line for that provider.

   The key never enters the model's context — the `!` prefix sends the command straight to the shell.

Output rules:

- For the Reuse branches (Case A Reuse, Case C Reuse where possible when all keys were reusable): print the binary's stdout verbatim.
- For Variant 1 / Variant 2 print blocks: print exactly as shown above. Do not add commentary after.
- Do not fix any issues the user mentions; the slash command is read-only on the codebase.
