{
  description = "auto-patcher Claude skills and agents";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    let
      # Package only the SKILL.md prompt files for installation.
      # Excludes Go source files (embed.go) and anything else alongside the prompts.
      # Downstream flakes can symlink $out/share/claude/skills/* into
      # ~/.claude/skills/ to make skills available as slash commands.
      mkSkillsPackage =
        pkgs:
        pkgs.runCommand "auto-patcher-skills" { } ''
          mkdir -p $out/share/claude/skills
          for skill in ${./skills}/*/SKILL.md; do
            name=$(basename $(dirname $skill))
            mkdir -p $out/share/claude/skills/$name
            cp $skill $out/share/claude/skills/$name/SKILL.md
          done
        '';

      # Package the autopatcher agent files (CLAUDE.md + PATCHER.md template).
      mkAgentPackage =
        pkgs:
        pkgs.runCommand "auto-patcher-agent" { } ''
          mkdir -p $out/share/auto-patcher
          cp ${./CLAUDE.md} $out/share/auto-patcher/CLAUDE.md
          cp ${./PATCHER.md} $out/share/auto-patcher/PATCHER.md
        '';

      # Build the dispatcher binary.
      # The dispatcher embeds the skill prompts at compile time via //go:embed.
      # Prerequisite: run `go mod tidy` at the repo root to generate go.sum,
      # then update vendorHash with the hash from the first failed `nix build`.
      mkDispatcherPackage =
        pkgs:
        pkgs.buildGoModule {
          pname = "dispatcher";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "." ];
          # Placeholder: run `nix build .#dispatcher` after `go mod tidy` to get
          # the real hash from the error output, then replace this value.
          vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
        };
    in
    {
      lib = {
        inherit mkSkillsPackage mkAgentPackage mkDispatcherPackage;
      };
    }
    // flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          default = mkSkillsPackage pkgs;
          agent = mkAgentPackage pkgs;
          dispatcher = mkDispatcherPackage pkgs;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # secrets
            sops
            age
            # container runtime
            podman
            # go toolchain
            go
            gopls
          ];
          shellHook = ''
            for skill in skills/*/SKILL.md; do
              name=$(basename $(dirname $skill))
              mkdir -p ".claude/skills/$name"
              ln -sfn "$(pwd)/$skill" ".claude/skills/$name/SKILL.md"
            done
            echo "auto-patcher dev shell"
            echo "skills:     /patch-init"
            echo "dispatcher: nix build .#dispatcher"
          '';
        };

        formatter = pkgs.nixfmt;
      }
    );
}
