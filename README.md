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
