# kizunax flag reference

Every flag accepted by `/kizunax:review` and `/kizunax:adversarial-review`,
grouped by purpose. Both slash commands share the same binary entry point
(`runReviewWithMode`), so anything documented here applies to both unless
noted otherwise.

For other slash commands (`/kizunax:status`, `/kizunax:result`,
`/kizunax:cancel`, `/kizunax:setup`, `/kizunax:usage`, `/kizunax:index`,
`/kizunax:expansion`, `/kizunax:context`), the slash command files in
[`plugins/kizunax/commands/`](../plugins/kizunax/commands/) are the
canonical reference — they're short.

---

## Target (pick at most one)

The diff scope. Default if none is set: `--working-tree`, auto-flipped to
`--base auto` when the working tree is clean (v0.21.0+).

| Flag | Since | Description |
|---|---|---|
| `--working-tree` | v0.1 | Review uncommitted changes (staged + unstaged + untracked text files). |
| `--base <ref>` | v0.1 | Branch diff vs `<ref>` — typically `develop` or `main`. v0.20.0+ accepts `--base auto`, which resolves to upstream tracking branch with fallback chain (develop → dev → main → master → origin/HEAD). |
| `--commit <sha>` | v0.1 | Review a single commit. |
| `--from <sha> --to <sha>` | v0.1 | Review a commit range. |

Examples:

```bash
/kizunax:review --working-tree
/kizunax:review --base develop
/kizunax:review --base auto                 # smart upstream detection
/kizunax:review --commit HEAD~1
/kizunax:review --from v1.5.0 --to v1.5.1
```

---

## Behavior

| Flag | Since | Default | Description |
|---|---|---|---|
| `--strategy auto\|single\|fanout` | v0.26.0 | `auto` | Routes diff through binary's internal fan-out for large diffs. `auto` decides per size (>150 KB or >100 files → fan-out). `single` forces one LLM call. `fanout` forces parallel buckets even on small diffs. |
| `--paths a,b/` | v0.1 (resolver scope v0.28.0) | unset | Comma-separated path filter. Limits both the diff scope AND, since v0.28.0, the resolver workspace walk to the union of these subtrees. Use on monorepos to avoid the 3000-file `WorkspaceFileCap` auto-skip. |
| `--focus "text"` | v0.1 | unset | Focus hint prepended into the prompt. Trailing free-form text after target flags is also captured as focus (e.g., `/kizunax:adversarial-review --base main challenge the cache design`). |
| `--model <name>` | v0.27.0 | (from config) | Override the configured model for one invocation. Example: `--model claude-opus-4-7`. |
| `--add-context-prompt` | v0.27.0 | off | Slash command asks one AskUserQuestion for per-review inline context, base64-encodes the answer into `--context-text`. NOT persisted to `.kizunax/review-context.md` — that's the role of `/kizunax:context`. |
| `--provider openai\|anthropic` | v0.6 | (from config) | Override provider routing for one invocation. |
| `--quiet` | v0.6 | off | Suppress the usage-warning footer. Useful when piping output. |
| `--verbose` | v0.6 | off | Log timing, model, provider, extractor path, glossary / review-context sizes to stderr. |

Examples:

```bash
# Fan-out a 280 KB diff into parallel buckets
/kizunax:review --base develop --strategy=fanout

# Narrow scope + add focus hint
/kizunax:review --base develop --paths app/Http,app/Services --focus "race conditions in approval flow"

# Per-review inline context (asks once)
/kizunax:review --base develop --add-context-prompt
```

---

## Expansion (workspace symbol enrichment)

These flags toggle whether the engine reads referenced workspace files
into the prompt. Default behavior: enrichment on, capped by
`EnrichBudget` (32 / 96 KiB), auto-skipped on workspaces > 3000 tracked
files (v0.19.1+; the cap is hard-coded in `runner.go` for now — a
configurable override is on the roadmap).

| Flag | Since | Default | Description |
|---|---|---|---|
| `--no-expand` | v0.1 | off | Skip enrichment entirely. Use for huge monorepos or fast pre-commit pass. |
| `--expand-callers` | v0.11 | off | Include callers of diff symbols. |
| `--expand-typedefs` | v0.11 | off | Include type definitions referenced by diff symbols. |
| `--expand-tests` | v0.11 | off | Include test files touching diff symbols. |
| `--expand-all` | v0.11 | off | Enable all three expansion strategies. |
| `--rescan` | v0.13 | off | Force re-scan of workspace index even when warm. |
| `--use-index` | v0.28.0 | off | Opt into the v2 AST-backed resolver. Requires a pre-built index — run `kizunax index sync` once per workspace, then every subsequent `/kizunax:review --use-index` uses the AST resolver instead of v1 regex BFS. Kill switch: `KIZUNAX_DISABLE_INDEX=1`. Env fallback: `KIZUNAX_USE_INDEX=1`. On big monorepos this is the path to fast, accurate enrichment. |
| `--summary` / `--no-summary` | v0.11 | (mode-dependent) | Force TL;DR summary on / off. Mutually exclusive. |

Examples:

```bash
# Pre-commit pass — skip enrichment for speed
/kizunax:review --working-tree --no-expand --strategy=single

# Deep review — expand everything
/kizunax:review --base develop --expand-all --strategy=fanout
```

---

## Internal flags

Used by the slash command layer; users normally don't pass these directly.

| Flag | Set by | Description |
|---|---|---|
| `--context-text <base64>` | `/kizunax:review --add-context-prompt` | Inline per-review context, base64-encoded so it survives shell quoting. The binary decodes + appends to file-based review-context. |

---

## Environment variables

| Env var | Since | Default | Description |
|---|---|---|---|
| `KIZUNAX_PHP_EXTRACTOR` | v0.25.0 | `auto` | Override PHP extraction strategy. Values: `auto`, `phpsyms`, `treesitter`, `regex`. `auto` prefers Go-native [phpsyms](https://github.com/thanhhaudev/phpsyms) and falls back to tree-sitter only on empty result. |
| `KIZUNAX_BUNDLE_LOG` | v0.12 | unset | Set to `1` to write JSONL telemetry to `~/.kizunax/state/<ws-hash>/bundle.log` (capped 10 MiB). Use for offline post-mortems. |
| `KIZUNAX_PROVIDER` | v0.6 | unset | Default provider when `--provider` isn't passed. Values: `openai`, `anthropic`. |
| `KIZUNAX_API_KEY` | v0.6 | unset | Override the API key resolved from config. Useful for one-off testing with a fresh key. |

---

## Deprecated flags

| Flag | Deprecated since | Replacement |
|---|---|---|
| `--wait` | v0.9 | No replacement needed. Background execution is now delegated to Claude Code's `Bash(run_in_background:true)` at the slash-command layer; binary is always foreground. |
| `--background` | v0.9 | Same as above. The binary accepts this for back-compat but logs a deprecation note to stderr. |

---

## Notes on related artifacts

| File | Read by | When |
|---|---|---|
| `~/.kizunax/config.json` | Every review | Provider + model + API keys + base URL. Edit via `/kizunax:setup`. |
| `.kizunax/review-context.md` | Every review | Behavioral hints injected above the system prompt. Build via `/kizunax:context`. v0.27.0+. |
| `.kizunax/glossary.md` | Every review | Project vocabulary injected as "Project glossary" section. v0.11+. |
| `CLAUDE.md` (workspace root) | `/kizunax:context` only | One of the synthesis sources when generating `review-context.md`. Not read directly by `/kizunax:review`. |
| `~/.kizunax/state/<ws-hash>/index/index.json` | `/kizunax:review --use-index` | When set, runner loads this AST symbol index instead of running the v1 regex BFS. Build via `kizunax index sync` (one-shot) or let `--rescan` rebuild on the next review. Stale-threshold 24h. |

---

## Cross-reference

- Slash command source (CC contract): [`plugins/kizunax/commands/`](../plugins/kizunax/commands/)
- Binary entry point: [`internal/cli/cmd_review.go::runReviewWithMode`](../internal/cli/cmd_review.go)
- Strategy decision: [`internal/cli/cmd_review.go`](../internal/cli/cmd_review.go) (search `shouldFanout`)
- Enrichment gate: [`llmreviewkit/engine/helpers.go::shouldEnrich`](https://github.com/thanhhaudev/llmreviewkit/blob/master/engine/helpers.go)
