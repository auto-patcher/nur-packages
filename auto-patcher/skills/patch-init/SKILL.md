Initialize a new autopatcher-managed fork of an upstream repository.

Arguments: `$ARGUMENTS` — the upstream to fork, as `owner/repo` or a full GitHub URL.

After initialization, the fork is registered for automated cycle management by
the dispatcher. Skills are embedded in the dispatcher binary — there is nothing
to copy into the fork.

---

## Step 1 — Parse arguments

Extract `owner/repo` from `$ARGUMENTS`, stripping any `https://github.com/` prefix.

Derive:
- `upstream`: `owner/repo`
- `repo_name`: the repo part
- `fork`: `auto-patcher/<repo_name>`
- `local_path`: `~/code/auto-patcher/<repo_name>`

## Step 2 — Fork to auto-patcher org

Use the GitHub MCP `fork_repository` tool to fork `<upstream>` into the
`auto-patcher` organization. If a fork already exists there, note it and continue.

## Step 3 — Clone locally

```bash
gh repo clone auto-patcher/<repo_name> ~/code/auto-patcher/<repo_name>
```

## Step 4 — Fetch upstream metadata

Using GitHub MCP tools, fetch the latest release tag from the upstream repo.
This becomes the initial `last_patched` value — the fork is considered current
as of the fork point. The next dispatcher cycle will analyze everything released
after this tag.

## Step 5 — Scaffold autopatcher files

Push the following files to `auto-patcher/<repo_name>` via GitHub MCP tools.

### `CLAUDE.md`

Fetch from `auto-patcher/skills` and copy verbatim. This is the shared agent
definition — do not modify it.

### `PATCHER.md`

Before writing `PATCHER.md`, **read the codebase** to generate meaningful
content. Use the GitHub MCP tools to explore the fork:

1. Fetch the repository tree to understand the top-level structure
2. Read key files: `README.md`, `CHANGELOG.md`, any docs directory, build files
   (`Makefile`, `flake.nix`, `package.json`, `Cargo.toml`, `go.mod`, etc.), and
   CI configuration (`.github/workflows/`)
3. Sample source files from the main language directories to understand naming
   conventions, idioms, and patterns
4. Look for test files and test runner configuration to understand the testing setup
5. Read the upstream `README.md` and compare with the fork's — differences reveal
   purpose and divergence

From this analysis, produce a filled-out `PATCHER.md`. Write real sentences based
on what you found. Do not leave placeholder comments — if you genuinely cannot
determine something, write a brief note explaining what is unknown.

```markdown
# Patcher

## Repositories

```
upstream: <upstream>
fork:     auto-patcher/<repo_name>
```

## Upstream baseline

```
last_patched: <latest_upstream_release_tag>
```

The upstream version tag last incorporated into this fork. The next dispatcher
cycle will analyze everything released after this.

## Purpose

<derived from README diff, fork description, and any docs>

## Character

<derived from reading the code — what trade-offs does this fork make, what does
a developer native to it care about>

## Architecture

<derived from the repo tree and source files — concrete structural divergences
from upstream: renamed packages, replaced dependencies, removed or added
subsystems, key files that differ>

## Style

<derived from sampling source files — naming conventions, idioms, formatting
patterns specific to this fork>

## Testing

### Unit tests

<command derived from Makefile / CI / test runner config>

### Integration tests

<command and required setup derived from CI or docs; note "not found" if absent>

### Build

<command derived from build files>

### Smoke tests

<concrete manual checks derived from README or docs; note "not determined" if absent>

### Subagent testing

<any scenarios that benefit from a subagent — derived from the project's nature>
```

## Step 6 — Report

Summarize what was done:
- Fork URL: `https://github.com/auto-patcher/<repo_name>`
- Local clone: `~/code/auto-patcher/<repo_name>`
- Initial `last_patched`: `<tag>`
- Files added: `CLAUDE.md`, `PATCHER.md`

Review the generated `PATCHER.md` with the user. Highlight any sections where
you had low confidence — missing docs, sparse README, ambiguous test setup —
so they know what to verify before the dispatcher picks up this repo. The
Testing section in particular must be accurate; the dispatcher uses it to verify
work at every stage of the patch cycle.
