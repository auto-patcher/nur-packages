# Patcher

<!--
  This file is the autopatcher's context for this fork — analogous to CLAUDE.md but
  specific to the patching agent. Write it as a description of who this fork is, not
  a log of what has happened to it. The agent reads this at the start of every session
  to understand what it is working on and what must be preserved.
-->

## Repositories

```
upstream: 
fork:     
```

## Upstream baseline

```
last_patched: 
```

The upstream version tag last incorporated into this fork. The next `/patch-dissect` run will analyze everything released after this.

## Purpose

<!-- Why does this fork exist? What does it do that upstream doesn't, or differently?
     Write this as a short statement of intent — what problem this fork solves and for whom. -->

## Character

<!-- Describe how this fork thinks and works. What are its values? What trade-offs does it
     make differently from upstream? What would a developer native to this codebase care about?
     This is what the agent uses to judge whether a patch "fits". -->

## Architecture

<!-- Where does this fork diverge structurally from upstream? Different module layout,
     replaced dependencies, new layers or abstractions, removed subsystems?
     Be concrete — name the packages, files, or patterns that differ. -->

## Style

<!-- Naming conventions, idioms, patterns, and preferences specific to this fork.
     The agent uses this when deciding how to express an upstream feature in fork terms
     rather than copying the upstream implementation. -->

## Testing

<!-- How to verify that this fork is working correctly. The agent is expected to test
     from multiple angles before closing any issue or cutting a release. Be specific —
     include commands, expected outputs, and what "working" looks like for this fork. -->

### Unit tests

<!-- Command to run the unit test suite, e.g.:
     go test ./...
     bun test
     pytest -->

### Integration tests

<!-- Command to run integration or end-to-end tests, if separate from unit tests.
     Note any required setup (env vars, running services, test fixtures). -->

### Build

<!-- How to build the project from source, e.g.:
     go build ./...
     bun run build
     nix build -->

### Smoke tests

<!-- What to do manually after a build to verify basic functionality.
     Think: what are the first 3 things you'd try if you just installed this?
     Include concrete commands or interactions, not just "check that it works".

     Example:
     - Run `./bin/tool --help` and verify the help text renders
     - Run `./bin/tool <basic-command>` and verify expected output
     - Try the feature most likely to break under a patch -->

### Subagent testing

<!-- If any test scenarios benefit from a subagent (e.g. testing a server by having
     an agent act as a client, testing an AI tool by running it against a sample task),
     describe how to set that up here. -->
