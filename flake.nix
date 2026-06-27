{
  description = "auto-patcher NUR package set";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f:
        nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      # `packages.<system>` — the flat package set, with every entry a
      # derivation, plus a `default` that builds the whole set at once.
      packages = forAllSystems (pkgs:
        let
          all = import ./default.nix { inherit pkgs; };
          drvs = nixpkgs.lib.filterAttrs (_: v: nixpkgs.lib.isDerivation v) all;
        in
        drvs // {
          default = pkgs.symlinkJoin {
            name = "auto-patcher-nur-all";
            paths = nixpkgs.lib.attrValues drvs;
          };
        });

      # Overlay so the package set can be merged into an existing nixpkgs,
      # consumed directly as a flake input without going through NUR.
      overlays.default = final: _prev: import ./default.nix { pkgs = final; };

      # `legacyPackages` mirrors `default.nix` verbatim — this is the entry
      # point the NUR community infra evaluates.
      legacyPackages = forAllSystems (pkgs: import ./default.nix { inherit pkgs; });
    };
}
