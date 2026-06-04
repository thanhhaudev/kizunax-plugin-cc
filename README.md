# kizunax-plugin-cc

A Claude Code plugin that runs AI code reviews through any OpenAI- or
Anthropic-compatible endpoint. Single Go binary, no extra CLI.

> The repo is named after [KizunaX](https://kizunax.io), the API provider
> it was first built against. The plugin works with any compatible
> endpoint; KizunaX is not required.

> A personal research and learning project, built to study how LLM APIs
> and Claude Code plugins fit together.

## How it works

```
Claude Code  ──/kizunax:review──▶  plugins/kizunax/bin/kizunax
                                          │
                                          ▼
                                  llmreviewkit engine
                                  (diff → prompt → JSON → markdown)
                                          │
                                          ▼
                                  any OpenAI- or Anthropic-compatible
                                  HTTP endpoint
                                          │
                                          ▼
                                  markdown back to Claude
```

Engine details live in
[llmreviewkit](https://github.com/thanhhaudev/llmreviewkit). The plugin
adds: slash commands, multi-provider config with a shared key pool,
background jobs, hooks (`SessionStart` / `SessionEnd` cleanup + optional
`Stop` review-gate), and a setup wizard.

## Compared to codex-plugin-cc

Mechanism, not features:

| | kizunax-plugin-cc | codex-plugin-cc |
|---|---|---|
| Execution | single LLM request | multi-turn agent loop |
| Transport | direct HTTP to the model API | spawns the Codex CLI subprocess |
| Diff handling | pre-packed bundle (diff + optional refs) | agent reads files on demand |
| Output enforcement | enforced JSON schema / `tool_use` | Codex CLI structured output |
| Token budget | capped at `EnrichBudget` (32 / 96 KiB) | no cap |
| Latency | ~one inference call | varies with task; can be minutes |

Use kizunax for fast, predictable reviews of a specific change; use codex when the model needs to read beyond the diff.

## Install

macOS / Linux:

```bash
git clone https://github.com/thanhhaudev/kizunax-plugin-cc.git
cd kizunax-plugin-cc
./install.sh
```

Windows (PowerShell):

```powershell
git clone https://github.com/thanhhaudev/kizunax-plugin-cc.git
cd kizunax-plugin-cc
.\install.ps1
```

Requires `git`, `curl`, and one of `jq` / `python3` (macOS / Linux) or
PowerShell 5.1+ (Windows). Go 1.21+ only if no pre-built binary matches
your platform.

## Setup

From the repo root:

```bash
./setup.sh
```

Or, inside Claude Code:

```
/kizunax:setup
```

Both open a local browser form for provider, base URL, model, and API
key(s). Saved to `~/.kizunax/config.json` (`%USERPROFILE%\.kizunax\config.json`
on Windows).

Other flags `./setup.sh` accepts: `--status`, `--enable-stop-gate`,
`--disable-stop-gate`.

## Commands

| Command | What it does |
|---|---|
| `/kizunax:review` | Review changes against working tree, a branch base, a commit, or a range. `--wait` / `--background`, `--paths`, `--focus`. |
| `/kizunax:adversarial-review` | Same targets as `review`, but the model challenges the design and looks for risks. Accepts trailing focus text. |
| `/kizunax:status` | List background review jobs. |
| `/kizunax:result <id>` | Print the result of a finished job. |
| `/kizunax:cancel <id>` | Cancel a running background job. |
| `/kizunax:setup` | Re-configure provider, model, or API keys. |
| `/kizunax:usage` | Print per-key quota table (KizunaX provider only). |
| `/kizunax:index <sub>` | Manage the workspace AST index. |
| `/kizunax:expansion <sub>` | Toggle bundle expansion strategies. |

Per-command flags in `plugins/kizunax/commands/`.

## Examples

```
# Review the working tree (default target)
/kizunax:review

# Review the latest commit, in the background
/kizunax:review --commit HEAD --background
/kizunax:status
/kizunax:result <id>

# Review the current branch against main
/kizunax:review --base main

# Review only specific paths
/kizunax:review --paths internal/auth,internal/handlers --focus "race conditions"

# Adversarial review with trailing focus text
/kizunax:adversarial-review --base main challenge the cache invalidation strategy
```

## Uninstall

macOS / Linux:

```bash
./uninstall.sh
```

Windows (PowerShell):

```powershell
.\uninstall.ps1
```

Both remove the plugin from `settings.json` and delete the binary. Your
config and job history under `~/.kizunax/` stay; delete by hand for a
full reset.

---

Inspired by [codex-plugin-cc](https://github.com/openai/codex-plugin-cc).
