# Meta package: the deterministic generator that maintains this NUR. Built with
# Nix so it is itself part of the package set
# (nur.repos.auto-patcher.auto-packager) and verified by CI alongside the
# auto-patched packages.
#
# The Go module lives in its own tree at tools/auto-packager/{go.mod, *.go, ...}.
{ lib, buildGoModule }:

buildGoModule {
  pname = "auto-packager";
  version = "0.1.0";

  src = ../../tools/auto-packager;

  # Pure standard library, no module dependencies.
  vendorHash = null;

  meta = {
    description = "Deterministic generator for the auto-patcher NUR package set";
    homepage = "https://github.com/auto-patcher/nur-packages";
    license = lib.licenses.mit;
    platforms = lib.platforms.all;
    mainProgram = "auto-packager";
  };
}
