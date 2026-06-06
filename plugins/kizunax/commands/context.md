---
description: Build or refresh .kizunax/review-context.md from project sources (CLAUDE.md, memory, conversation)
argument-hint: '[--workspace <path>]'
disable-model-invocation: true
allowed-tools: Read, Bash(ls:*), Bash(git:*), Write, AskUserQuestion
---

You will build a review-context file for kizunax by synthesizing
project-specific knowledge into ~6 KiB of markdown.

The OUTPUT file is `.kizunax/review-context.md` at the current workspace
root. kizunax binary auto-injects this file's content into every review's
system prompt.

Raw slash-command arguments:
`$ARGUMENTS`

## Step 1 — Gather sources

Read these sources if they exist; skip silently if missing. Treat every
read as best-effort, never block on absent files.

### Tier 1 (always check)

1. **`CLAUDE.md`** at workspace root — use the `Read` tool.
2. **Existing `.kizunax/review-context.md`** — use `Read`. If present, this
   is a refresh: show a diff at Step 3.
3. **Current conversation** — already in your context window. Reflect on
   what intentional patterns, suppressed categories, or business constraints
   the user has discussed in this session.

### Tier 2 (helpful when present)

4. **Memory dir of current workspace**:
   - Slug = current cwd with all `/` replaced by `-`. Example:
     `/Users/haunguyen/Documents/Bear/Oneplat-B2B-System` →
     `-Users-haunguyen-Documents-Bear-Oneplat-B2B-System`.
   - Path: `~/.claude/projects/<slug>/memory/`
   - First check existence: `Bash(ls -d ~/.claude/projects/<slug>/memory/ 2>/dev/null)`.
     If the path does not exist (common for new or low-traffic workspaces),
     skip this source silently and continue to source #5.
   - If dir exists, use `Bash(ls)` to list `project_*.md` files only. Filter by
     filename prefix `project_` — this is the canonical filter, NOT by YAML
     frontmatter. Skip `feedback_*.md`, `user_*.md`, `reference_*.md`, and
     `MEMORY.md` by NOT matching them.

5. **Cross-project memory references** — solves the "memory at conversation
   location" issue (a `project_oneplat_workflow.md` may live in the kizunax
   memory dir because the conversation about Oneplat happened there):
   - List `~/.claude/projects/` with `Bash(ls)`.
   - For each project dir, list its `memory/project_*.md` files.
   - For each candidate, `Read` enough to check the `description` field.
   - Match rule: include the file IF its `description` field contains the
     workspace's directory basename as a substring (case-sensitive,
     basename ≥ 4 chars). Skip basenames shorter than 4 chars (e.g. `api`,
     `web`, `cli`) — too ambiguous, would cause false matches.
     - Example: cwd `/Users/haunguyen/Documents/Bear/Oneplat-B2B-System` →
       basename `Oneplat-B2B-System` → include any project_*.md whose
       description contains exact string `Oneplat-B2B-System`.
   - Hard cap: scan up to 20 dirs, 5 candidate files per dir. Stop at
     first 3 matches to avoid blowing prompt budget.

6. **Recent git activity** — `Bash(git log --oneline -20)` to spot current
   work focus.

7. **Language/framework pins** — `Read` whichever of these exist:
   - `composer.json` (PHP version, Laravel version)
   - `package.json` (Node version, framework deps)
   - `.python-version`, `.ruby-version`, `.go-version`
   - `go.mod`

## Privacy filter — NEVER include

- Files matching `feedback_*.md`, `user_*.md`, `reference_*.md`.
- API keys, tokens, passwords, secrets in environment-style content.
- URLs containing credentials (e.g., `https://user:pass@host/`).
- Connection strings.

If you encounter such content while reading, redact it before adding to
the synthesis.

## Step 2 — Synthesize

Produce a markdown document with this exact structure:

```markdown
# Review context for <repo-name>

## Intentional patterns (do NOT flag)
- <bullets — patterns from CLAUDE.md, conversation, memory>

## Suppressed categories
- <bullets — noise the reviewer should skip>

## Business context
- Framework: <e.g. Laravel 7 / PHP 7.4>
- Current focus: <from recent conversation + git log>
- <other constraints>

<!-- Source attribution -->
<!-- from CLAUDE.md section: <name> -->
<!-- from conversation (this session) -->
<!-- from ~/.claude/projects/.../project_oneplat_workflow.md -->
```

Constraints:
- Total length under 6 KiB (~3000 tokens). Be specific, not verbose.
- **Bullet count target**: 6-10 most-impactful items per section. Quality over
  completeness. If you have 15 candidate bullets, pick the 8 with highest
  impact on reducing false-positive review findings. Omit speculative or
  low-value items.
- Every bullet should be actionable — "the reviewer should skip X" or
  "treat Y as intentional". Avoid vague statements like "be aware of Z".
- Include HTML comment attribution lines so the user can verify provenance.

## Step 3 — Confirm

If an existing `.kizunax/review-context.md` is present, show the diff
between old and new versions in your response (use a unified-diff-style
markdown code block).

Then call AskUserQuestion:

- Header: `Save context`
- Question: `Generated N KB review-context. Save to .kizunax/review-context.md?`
- Options:
  - `Save as-is` — Recommended; this is the default
  - `Show full preview` — print the full synthesized markdown, then re-ask
  - `Edit before save` — tell the user to copy the draft, edit manually,
    and save themselves; do not Write
  - `Cancel` — abort

## Step 4 — Write

If the user chose `Save as-is`:
1. Ensure `.kizunax/` directory exists. If not, create via Write (Write
   tool auto-creates parent dirs on most systems; if Write fails, use
   `Bash(mkdir -p .kizunax)` then retry Write).
2. Use the Write tool to create `.kizunax/review-context.md`.
3. Print one line: `Saved N KB to .kizunax/review-context.md. Future reviews will auto-inject this context.`

If the user chose `Cancel` or `Edit before save`:
- Do not Write.
- Print the draft inline so the user can copy.

## Core constraint
- This command is review-context-only. Do not modify any other file in
  the workspace. Do not run code reviews. The ONLY Write target is
  `.kizunax/review-context.md`.
