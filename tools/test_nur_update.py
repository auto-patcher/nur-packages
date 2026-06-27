#!/usr/bin/env python3
"""Offline tests for the generator's pure logic (no nix, no network).

Run: python3 tools/test_nur_update.py
Exercises version parsing and the default.nix marker-block editing against a
temporary repo root via NUR_REPO_ROOT, so the real tree is never touched.
"""
import os
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
FAKE = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

DEFAULT_NIX_SEED = """{ pkgs ? import <nixpkgs> { } }:

{
  # BEGIN auto-patcher packages — managed automatically
  hello_1_0 = pkgs.callPackage ./pkgs/hello/1_0.nix { };
  # END auto-patcher packages
}
"""


def run_set(root: Path, repo: str, tag: str) -> None:
    env = {**os.environ, "NUR_REPO_ROOT": str(root)}
    subprocess.run(
        [sys.executable, str(HERE / "nur_update.py"), "set",
         "--repo", repo, "--tag", tag, "--hash", FAKE],
        check=True, capture_output=True, text=True, env=env,
    )


def main() -> int:
    # Unit-level: version parsing.
    sys.path.insert(0, str(HERE))
    import nur_update as nu  # noqa: E402

    assert nu.parse_version("v1.2.3") == ("1.2.3", 1, 2)
    assert nu.parse_version("1.2") == ("1.2", 1, 2)
    assert nu.parse_version("v2") == ("2", 2, 0)
    for bad in ("main", "release-x", ""):
        try:
            nu.parse_version(bad)
            raise AssertionError(f"expected failure for {bad!r}")
        except ValueError:
            pass

    # Integration-level: default.nix editing in a temp root.
    with tempfile.TemporaryDirectory() as d:
        root = Path(d)
        (root / "pkgs" / "hello").mkdir(parents=True)
        (root / "default.nix").write_text(DEFAULT_NIX_SEED)

        run_set(root, "widget", "v1.2.3")
        run_set(root, "widget", "v1.2.9")   # same major_minor -> overwrite
        run_set(root, "alpha", "3.0")        # sorts before widget, after hello

        out = (root / "default.nix").read_text()
        entries = [l.strip() for l in out.splitlines()
                   if "pkgs.callPackage" in l]
        assert entries == [
            "alpha_3_0 = pkgs.callPackage ./pkgs/alpha/3_0.nix { };",
            "hello_1_0 = pkgs.callPackage ./pkgs/hello/1_0.nix { };",
            "widget_1_2 = pkgs.callPackage ./pkgs/widget/1_2.nix { };",
        ], entries
        # exactly one widget entry despite two updates (idempotent key)
        assert sum("widget_1_2" in e for e in entries) == 1
        # latest hash/version landed in the version file
        vf = (root / "pkgs" / "widget" / "1_2.nix").read_text()
        assert 'version = "1.2.9";' in vf
        assert FAKE in vf
        # build.nix created once and is the real recipe (not the thin file)
        assert (root / "pkgs" / "widget" / "build.nix").exists()

    print("ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
