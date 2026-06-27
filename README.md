# auto-patcher/nur-packages

A [NUR](https://github.com/nix-community/NUR) (Nix User Repository) package set
for the `auto-patcher` org. Package definitions are generated automatically by
an external auto-patcher, one derivation per `<package>` `<version>`.

Packages are named `<pkg>_<major>_<minor>` (e.g. `foo_1_2`) and live in a flat
namespace. Consumers reach them as `nur.repos.auto-patcher.<pkg>_<major>_<minor>`.

## Binary cache

Build results are pushed to the public [Cachix](https://app.cachix.org/cache/auto-patcher)
cache `auto-patcher` by CI, so consumers fetch binaries instead of rebuilding
from source (NUR is evaluated by the community infra but not built by Hydra).
Add it as a substituter:

```nix
nix.settings = {
  substituters = [ "https://auto-patcher.cachix.org" ];
  trusted-public-keys = [ "auto-patcher.cachix.org-1:<public-key-from-app.cachix.org>" ];
};
```

or ad hoc: `cachix use auto-patcher`. The public key is shown on the cache page.

## Consuming

### Via the NUR overlay

Once this repo is registered with NUR, add NUR as usual and reference packages
by their flat name:

```nix
{ pkgs ? import <nixpkgs> { } }:

let
  nur = import (builtins.fetchTarball "https://github.com/nix-community/NUR/archive/main.tar.gz") {
    inherit pkgs;
  };
in
nur.repos.auto-patcher.foo_1_2
```

Or wire it in as an overlay / `packageOverrides` per the
[NUR installation docs](https://github.com/nix-community/NUR#installation).

### As a direct flake input (no NUR)

This repo is a self-contained flake, so you can depend on it directly without
pulling in NUR:

```nix
{
  inputs.auto-patcher.url = "github:auto-patcher/nur-packages";

  outputs = { self, nixpkgs, auto-patcher }: {
    # the package set for your system
    # auto-patcher.packages.${system}.foo_1_2

    # or merge everything into nixpkgs via the overlay
    # nixpkgs.legacyPackages.${system} extended with auto-patcher.overlays.default
  };
}
```

Flake outputs:

- `packages.<system>.<pkg>_<major>_<minor>` — each package as a derivation.
- `packages.<system>.default` — builds the entire set at once.
- `overlays.default` — merges the package set into an existing nixpkgs.
- `legacyPackages.<system>` — mirrors `default.nix` (the NUR evaluation entry point).

## Layout

```
default.nix                 # flat namespace consumers see (generated/append-friendly)
flake.nix                   # flake wrapper: packages / overlay / legacyPackages
pkgs/<name>/<version>.nix   # individual callPackage-style derivations
```

Every package sets `meta` (description + license, optionally platforms) so it is
indexed by NUR search. Packages that cannot build are kept with
`meta.broken = true` rather than deleted.

## Auto-patcher pipeline

Packages are generated and updated by deterministic code in `tools/` — **no LLM
in the update loop**. There are two entry points and two ways they fire.

### The generator (`tools/nur_update.py`)

```
python3 tools/nur_update.py set --repo <name> --tag <tag>   # one package
python3 tools/nur_update.py sync-all                        # every org repo -> latest tag
```

`set` parses the tag (`v1.2.3` → `1.2.3`, major `1`, minor `2`), prefetches the
source hash with `nix-prefetch-url --unpack`, stamps the version file, and edits
`default.nix`. It is idempotent and re-runnable. `sync-all` walks every repo in
the org (via `gh`), resolves each one's latest numeric tag, and updates them all.

Each package owns a version-agnostic recipe at `pkgs/<name>/build.nix`, written
once when the repo is onboarded and **never overwritten** by the generator. The
generator only writes the thin, per-version `pkgs/<name>/<major>_<minor>.nix`
(which passes `version` + `rev` + `hash` into `build.nix`) and the one managed
line in `default.nix`. Customize a package's real build by editing its
`build.nix`; the default template is a passthrough that installs the source.

### How it fires

- **On a new tag (event-driven).** A source repo adds the init template
  `templates/source-repo/on-tag.yml` (→ its `.github/workflows/on-tag.yml`). On
  every tag push it calls this repo's reusable `notify-nur.yml`, which sends a
  `package-tagged` `repository_dispatch` to nur-packages. That triggers
  `update.yml` here, which regenerates the package, verifies it builds, and
  commits to the default branch.
- **On a schedule (reconcile).** `sync.yml` runs `sync-all` daily (and on
  demand) to catch anything a dispatch missed.

```
source repo (tag push)
  └─ on-tag.yml ──uses──▶ nur-packages/notify-nur.yml ──dispatch──▶ nur-packages/update.yml
                                                                      └─ nur_update.py set → build → commit
nur-packages/sync.yml (cron) ─────────────────────────────────────▶ nur_update.py sync-all → build → commit
```

### Onboarding a repo

1. Add `templates/source-repo/on-tag.yml` to the repo as
   `.github/workflows/on-tag.yml`.
2. Ensure the `NUR_DISPATCH_TOKEN` secret is available to it (an org secret is
   easiest) and that nur-packages allows its reusable workflows to be called by
   org repos.
3. Tag a release — or run the `sync all` / `update package` workflow here
   manually with the repo + tag. The first run creates `pkgs/<name>/build.nix`;
   edit it to add the real build and license.

## Auto-patcher contract with `default.nix`

`default.nix` is the integration point. The auto-patcher manages the lines
between these markers:

```nix
  # BEGIN auto-patcher packages — managed automatically, keep entries one-per-line
  ...
  # END auto-patcher packages
```

For each package version it emits exactly one line, of the form:

```nix
  <pkg>_<major>_<minor> = pkgs.callPackage ./pkgs/<pkg>/<major>_<minor>.nix { };
```

and writes the corresponding derivation to `pkgs/<pkg>/<major>_<minor>.nix` (a
`callPackage`-style function: `{ lib, ... }: <derivation>` that sets `meta`).

Rules:

- The attribute name is the flat `<pkg>_<major>_<minor>` form; the file path
  uses the bare package name plus the `<major>_<minor>` version
  (e.g. `foo_1_2` → `./pkgs/foo/1_2.nix`).
- One line per entry, inside the marker block. The block stays a flat attrset of
  derivations only.
- Always set `meta.description` and `meta.license`. Mark unbuildable packages
  `meta.broken = true` instead of removing them.

The `hello_1_0` / `hello_2_0` entries are trivial seed examples demonstrating the
layout and a working build; they can be removed once real packages land.
