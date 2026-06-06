# kizunax-plugin-cc

A Claude Code plugin that runs AI code reviews against the
[KizunaX](https://kizunax.io) API. Single Go binary, no extra CLI.

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
                                  KizunaX API
                                  (OpenAI- or Anthropic-compatible
                                   chat completions)
                                          │
                                          ▼
                                  markdown back to Claude
```

Engine details live in
[llmreviewkit](https://github.com/thanhhaudev/llmreviewkit). The plugin
adds: slash commands, config with OpenAI- and Anthropic-compatible
slots and a shared key pool, background jobs, hooks (`SessionStart` /
`SessionEnd` cleanup + optional `Stop` review-gate), and a setup wizard.

## Compared to codex-plugin-cc

Mechanism, not features:

| | kizunax-plugin-cc | codex-plugin-cc |
|---|---|---|
| Execution | single LLM request | multi-turn agent loop |
| Transport | direct HTTP to the model API | spawns the Codex CLI subprocess |
| Diff handling | pre-packed bundle (diff + optional refs) | agent reads files on demand |
| Output enforcement | enforced JSON schema / `tool_use` | Codex CLI structured output |
| Token budget | capped at `EnrichBudget` (32 / 96 KiB) | no cap |
| Latency | ~one inference call (or N parallel calls with `--strategy=fanout` on large diffs) | varies with task; can be minutes |

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
| `/kizunax:review` | Review changes (working tree, branch base, commit, or range). Flags: `--strategy auto\|single\|fanout`, `--paths`, `--focus`, `--model`, `--add-context-prompt`. Accepts trailing free-form text as focus. |
| `/kizunax:adversarial-review` | Same as `review` (all flags + trailing focus text apply), but the model challenges the design and looks for risks. |
| `/kizunax:context` | Build or refresh `.kizunax/review-context.md` by synthesizing CLAUDE.md + memory + conversation. v0.27.0+. |
| `/kizunax:status` | List background review jobs. |
| `/kizunax:result <id>` | Print the result of a finished job. |
| `/kizunax:cancel <id>` | Cancel a running background job. |
| `/kizunax:setup` | Re-configure provider, model, or API keys. |
| `/kizunax:usage` | Print per-key quota table. |
| `/kizunax:index <sub>` | Manage the workspace AST index. |
| `/kizunax:expansion <sub>` | Toggle bundle expansion strategies. |

`--strategy=auto` (default since v0.26.0) routes large diffs through the
binary's internal fan-out (parallel review buckets merged into one report),
keeping the slash-command bash count to 1 per review. Full flag detail in
[docs/flag-reference.md](docs/flag-reference.md).

## Examples

```
# Review the working tree (default target)
/kizunax:review

# Review the current branch against develop (git-flow base)
/kizunax:review --base develop

# Review only specific paths
/kizunax:review --paths internal/auth,internal/handlers --focus "race conditions"

# Adversarial review with trailing focus text
/kizunax:adversarial-review --base main challenge the cache invalidation strategy

# Force fan-out for a large diff (parallel buckets + Anthropic)
/kizunax:review --base develop --strategy=fanout

# Override the configured model for one invocation
/kizunax:review --model claude-opus-4-7

# Per-review inline context hint (asks once via AskUserQuestion)
/kizunax:review --base develop --add-context-prompt

# Build .kizunax/review-context.md from CLAUDE.md + memory + conversation
/kizunax:context
```

## Configuration

Three files / one env var shape kizunax's behavior on top of
`~/.kizunax/config.json` (provider + API keys, written by `/kizunax:setup`):

| File | Purpose |
|---|---|
| `.kizunax/review-context.md` | Auto-injected above the system prompt: intentional patterns the reviewer should not flag, suppressed categories, business notes. Generate via `/kizunax:context`. v0.27.0+. |
| `.kizunax/glossary.md` | Project-specific vocabulary auto-injected as the "Project glossary" section. v0.11+. |
| `CLAUDE.md` (workspace root) | Already a Claude Code convention; `/kizunax:context` reads it as one of the sources when synthesizing review context. |

Environment variable:

- `KIZUNAX_PHP_EXTRACTOR=auto|phpsyms|treesitter|regex` — override the PHP
  extraction strategy. Default `auto` prefers Go-native
  [phpsyms](https://github.com/thanhhaudev/phpsyms) and falls back to
  tree-sitter on empty result. v0.25.0+.

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
