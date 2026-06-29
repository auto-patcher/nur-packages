Analyze upstream changes since `{{.LastPatched}}` and produce GitHub issues on
`{{.Repo}}` representing discrete, parallel units of backport work.

The repository is cloned in your current working directory. Read `PATCHER.md`
for this fork's purpose, character, architecture, and style — the mechanical
fields (fork, upstream, last_patched) are already provided in your context.

---

## Step 1 — Read PATCHER.md

Read `PATCHER.md`. Internalize the fork's purpose, character, architecture,
and style. You will use this in Step 3 to identify conflicts.

## Step 2 — Enumerate upstream versions

List all releases published on `{{.Upstream}}` after `{{.LastPatched}}`, in
chronological order. For each:
- Retrieve release notes
- Retrieve the list of commits and the full diff introduced by that release

## Step 3 — Semantic analysis, version by version

For each version hop (`vA → vB`), reason through the diff at a feature level:

- Identify discrete features, bug fixes, refactors, and breaking changes
- Group related commits into named changes. If a feature spans multiple commits
  or one commit sets up what a later one builds on, treat these as a **story** —
  a parent feature with a sequence of sub-points
- Note any changes that touch areas described in `PATCHER.md`. Flag these as
  potential conflicts
- Classify each change as `feature` or `bug`

Do not summarize commit messages. Read the code. Understand what changed and why.

## Step 4 — Construct a unified changelog

Collapse all version analyses into a single feature-level changelog:

    [change name]
      upstream version: v1.4.0
      type: feature | bug
      standalone | story
      description: ...
      upstream commits: ...
      fork conflict: yes/no — reason if yes

Order: stories first, then standalone features, then bug fixes.

## Step 5 — Create GitHub issues

For each entry in the changelog, create one GitHub issue on `{{.Repo}}`:

**Title**: `[backport] <change name> (upstream <version>)`

**Labels**: `backport` + `feature` or `bug`

**Body**:

    ## Summary
    <what this upstream change does>

    ## Upstream reference
    - version: <vX.Y.Z>
    - commits: <links>
    - PR/issue (if any): <link>

    ## Fork conflict
    <none | description of what in PATCHER.md this touches and why>

    ## Context
    <why this change exists upstream, what problem it solves>

For **stories**, create one parent issue with a task list (`- [ ] #<sub-issue>`)
and create the sub-issues individually.

For issues that depend on another being applied first, add `Depends on #N`
to the body.

Finally, post a single summary comment on all created issues containing the
full unified changelog, so every issue has the full picture as context.
