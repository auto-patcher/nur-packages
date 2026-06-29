Design the implementation for open issues on `{{.Repo}}`. For each eligible
issue, produce a design comment describing how to implement the change in this
fork's style, then mark it `ready`.

An issue is eligible if:
- It is open
- It was opened by a member of the `auto-patcher` org
- It does not already have the `ready` label
- It is not labeled `conflict`
- It is not labeled `human-review` (once the human removes the label the issue
  re-enters the pipeline as normal — check for any partial branch and factor it
  into the design comment)

This applies to any issue type — `backport`, `feature`, `bug`.

---

## Step 1 — Read PATCHER.md

Read `PATCHER.md` in the current working directory. Internalize:
- The fork's purpose, character, architecture, and style
- The Testing section — you will use this when writing testing plans

Then list all open, eligible issues on `{{.Repo}}`. Check issue authors against
the `auto-patcher` org member list.

## Step 2 — Order by dependency

- Issues with no dependencies come first
- If issue B says "Depends on #A", design A before B
- Story parent issues before their sub-issues

## Step 3 — Design each issue

For each eligible issue, in dependency order:

1. **Read** the issue body.
2. If `backport`: fetch the upstream code referenced in the issue. If `feature`
   or `bug`: read the description and any linked context carefully.
3. **Read** the corresponding area of the fork's codebase.
4. **Think**: how would a developer who wrote this fork implement this from scratch?

   Consider:
   - Does the proposed abstraction fit the fork's existing patterns?
   - Are there naming conventions in the fork that apply?
   - Does the fork already have partial infrastructure to extend?
   - Can this strengthen the fork's custom features?
   - Is the scope right, or should it be split or merged with existing code?

5. **Write a design comment** on the issue:

       ## Design

       ### Approach
       <how to implement this in the fork's style>

       ### Files to touch
       - `path/to/file.ext` — reason

       ### Risks
       <anything that could break existing fork behavior or conflict with PATCHER.md>

       ### Testing plan
       <all relevant angles from PATCHER.md's Testing section: unit tests,
       integration tests, build verification, smoke tests, subagent testing>

       ### Notes
       <anything else — e.g. if the upstream implementation is poor and we should do better>

6. **Add label** `ready` to the issue.

## Step 4 — Close no-ops

If the fork already has equivalent functionality, close the issue with a comment
explaining what existing code covers it. Add label `no-op` before closing.

## Step 5 — Flag conflicts

If an issue cannot be safely implemented without resolving a conflict with the
fork's character or architecture, add label `conflict` and post a comment
describing the conflict clearly. Do not write a design comment.
