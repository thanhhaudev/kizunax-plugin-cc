# KizunaX Plugin For Claude Code

Run AI code reviews in Claude Code via KizunaX or any other OpenAI/Anthropic-compatible endpoint.

## Requirements

- macOS, Linux, or Windows
- `git`, `curl`
- macOS / Linux: `jq` or `python3`
- Windows: PowerShell 5.1+ (ships with Windows 10 and 11)
- Go 1.21+ - only needed if no pre-built binary matches your platform

## Install

macOS / Linux:

```
git clone https://github.com/thanhhaudev/kizunax-plugin-cc.git
cd kizunax-plugin-cc
./install.sh
```

Windows (PowerShell):

```
git clone https://github.com/thanhhaudev/kizunax-plugin-cc.git
cd kizunax-plugin-cc
.\install.ps1
```

The script downloads the pre-built binary, falls back to `go build` if no matching binary is published, and patches `settings.json` for Claude Code. Restart Claude Code if it is already running.

## First-time setup

From the repo root:

```
./setup.sh
```

Or, from inside Claude Code:

```
/kizunax:setup
```

Both open the same local browser form. It asks for provider (`openai` or `anthropic`), base URL, model, and API key. The answers are saved to `~/.kizunax/config.json` (`%USERPROFILE%\.kizunax\config.json` on Windows).

`./setup.sh` also accepts pass-through flags: `--status` prints saved config + per-key probe results, `--enable-stop-gate` / `--disable-stop-gate` toggle the v0.7 review-on-stop hook.

## Commands

| Command | What it does |
|---------|--------------|
| `/kizunax:review` | Review code changes (working tree by default) |
| `/kizunax:adversarial-review` | Review with a skeptic stance focused on attack surface and failure modes |
| `/kizunax:status` | List background review jobs |
| `/kizunax:result <id>` | Print the result of a finished job |
| `/kizunax:cancel <id>` | Cancel a running background job |
| `/kizunax:setup` | Re-configure provider, model, or API key (or run `./setup.sh` from the shell) |
| `/kizunax:index <sub>` | Manage the v0.13 workspace AST index (`status`, `enable`/`disable`/`toggle`, `sync`, `purge`, `info <symbol>`) |
| `/kizunax:expansion <sub>` | Toggle v1.1.0 bundle expansion strategies (`status`, `enable`/`disable`/`set` `<csv>`, `reset`) |

Each command accepts flags. See `plugins/kizunax/commands/` for details.

## License

[MIT](LICENSE). Inspired by [codex-plugin-cc](https://github.com/openai/codex-plugin-cc).

## Uninstall

macOS / Linux:

```
./uninstall.sh
```

Windows (PowerShell):

```
.\uninstall.ps1
```

The script removes the plugin entries from `settings.json` and deletes the binary. Your config and job history under `~/.kizunax/` (`%USERPROFILE%\.kizunax\` on Windows) are left in place - delete them by hand if you want a full reset.
