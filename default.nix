# Package namespace for the `auto-patcher` NUR.
#
# This is the attrset consumers see through NUR, e.g.
#   nur.repos.auto-patcher.<pkg>_<major>_<minor>
#
# It is GENERATED / APPEND-FRIENDLY. The external auto-patcher manages the
# package entries between the marker comments below. Each entry is a single
# line of the exact form:
#
#   <pkg>_<major>_<minor> = pkgs.callPackage ./pkgs/<pkg>/<major>_<minor>.nix { };
#
# Keep this attrset flat: every value must be a derivation so the whole set
# stays buildable and indexable by NUR search. Do not add `lib`/`modules`/
# `overlays` helper attrs here unless they are also wired into CI.
{ pkgs ? import <nixpkgs> { } }:

{
  # BEGIN auto-patcher packages — managed automatically, keep entries one-per-line
  hello_1_0 = pkgs.callPackage ./pkgs/hello/1_0.nix { };
  hello_2_0 = pkgs.callPackage ./pkgs/hello/2_0.nix { };
  # END auto-patcher packages
}
