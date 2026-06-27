#!/usr/bin/env python3
"""Deterministic package generator for the auto-patcher NUR.

No LLM in this loop: given a source repo and a tag, this stamps a versioned
Nix derivation from a per-package template and edits `default.nix` so the
package shows up in the flat namespace. It also has a batch mode that walks
every repo in the org and brings each one up to its latest tag.

Subcommands:
  set        Update a single package to a given tag.
  sync-all   Update every org repo to its latest tag.

The source hash is computed with `nix-prefetch-url --unpack` (same unpacked
NAR hash fetchFromGitHub uses). Pass --hash to skip prefetch (used by tests).

File layout produced per package <repo>:
  pkgs/<repo>/build.nix          version-agnostic recipe, written ONCE, never
                                 overwritten — onboarding/humans own it.
  pkgs/<repo>/<major>_<minor>.nix  generated thin file: version + rev + hash.
  default.nix                    one managed line inside the marker block.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

# Repo root defaults to this file's parent's parent, but can be overridden with
# NUR_REPO_ROOT so the tool can run against a checkout elsewhere (and so tests
# can point it at a temporary fixture).
REPO_ROOT = Path(os.environ.get("NUR_REPO_ROOT") or Path(__file__).resolve().parent.parent)
PKGS_DIR = REPO_ROOT / "pkgs"
DEFAULT_NIX = REPO_ROOT / "default.nix"
TEMPLATES = Path(__file__).resolve().parent / "templates"

BEGIN_MARKER = "# BEGIN auto-patcher packages"
END_MARKER = "# END auto-patcher packages"

# A default.nix managed entry: `attr = pkgs.callPackage ./pkgs/<repo>/<mm>.nix { };`
ENTRY_RE = re.compile(
    r"^\s*(?P<attr>[A-Za-z_][A-Za-z0-9_.-]*)\s*=\s*"
    r"pkgs\.callPackage\s+\./pkgs/[^\s]+\.nix\s*\{\s*\};\s*$"
)

DEFAULT_OWNER = "auto-patcher"


# --------------------------------------------------------------------------- #
# version / naming helpers
# --------------------------------------------------------------------------- #
def parse_version(tag: str) -> tuple[str, int, int]:
    """Return (version_string, major, minor) from a tag like v1.2.3 / 1.2 / v2.

    The version string keeps the tag's numeric form (leading 'v' stripped);
    major/minor drive the attr name and file name. Missing minor defaults to 0.
    """
    m = re.match(r"^[vV]?(\d+)(?:\.(\d+))?(?:\.(\d+))?", tag.strip())
    if not m:
        raise ValueError(f"tag {tag!r} does not start with a numeric version")
    major = int(m.group(1))
    minor = int(m.group(2) or 0)
    nums = [g for g in (m.group(1), m.group(2), m.group(3)) if g is not None]
    version = ".".join(nums)
    return version, major, minor


def attr_name(repo: str, major: int, minor: int) -> str:
    return f"{repo}_{major}_{minor}"


def mm(major: int, minor: int) -> str:
    return f"{major}_{minor}"


# --------------------------------------------------------------------------- #
# hashing
# --------------------------------------------------------------------------- #
def archive_url(owner: str, repo: str, tag: str) -> str:
    return f"https://github.com/{owner}/{repo}/archive/refs/tags/{tag}.tar.gz"


def prefetch_hash(owner: str, repo: str, tag: str) -> str:
    """Compute the SRI hash of the unpacked tag tarball via nix.

    Mirrors fetchFromGitHub's unpacked-NAR hash, so the value can be dropped
    straight into the derivation.
    """
    url = archive_url(owner, repo, tag)
    base32 = subprocess.run(
        ["nix-prefetch-url", "--unpack", "--type", "sha256", url],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    # Newer nix: `nix hash convert`; older: `nix hash to-sri`.
    for cmd in (
        ["nix", "hash", "convert", "--hash-algo", "sha256", "--to", "sri", base32],
        ["nix", "hash", "to-sri", "--type", "sha256", base32],
    ):
        try:
            return subprocess.run(
                cmd, check=True, capture_output=True, text=True
            ).stdout.strip()
        except (subprocess.CalledProcessError, FileNotFoundError):
            continue
    raise RuntimeError("could not convert hash to SRI form")


# --------------------------------------------------------------------------- #
# template rendering
# --------------------------------------------------------------------------- #
def render(template_name: str, **fields: str) -> str:
    text = (TEMPLATES / template_name).read_text()
    for key, value in fields.items():
        text = text.replace(f"@{key}@", value)
    return text


def ensure_build_recipe(owner: str, repo: str) -> Path:
    """Create pkgs/<repo>/build.nix from the template if absent. Never clobber."""
    pkg_dir = PKGS_DIR / repo
    pkg_dir.mkdir(parents=True, exist_ok=True)
    build = pkg_dir / "build.nix"
    if not build.exists():
        build.write_text(render("build.nix.tmpl", OWNER=owner, REPO=repo))
    return build


def write_version_file(
    owner: str, repo: str, major: int, minor: int, version: str, tag: str, sri: str
) -> Path:
    pkg_dir = PKGS_DIR / repo
    pkg_dir.mkdir(parents=True, exist_ok=True)
    path = pkg_dir / f"{mm(major, minor)}.nix"
    path.write_text(
        render(
            "version.nix.tmpl",
            OWNER=owner,
            REPO=repo,
            TAG=tag,
            VERSION=version,
            REV=tag,
            HASH=sri,
        )
    )
    return path


# --------------------------------------------------------------------------- #
# default.nix editing
# --------------------------------------------------------------------------- #
def update_default_nix(repo: str, major: int, minor: int) -> bool:
    """Insert/replace the managed entry line. Idempotent. Returns True if changed."""
    attr = attr_name(repo, major, minor)
    line = f"  {attr} = pkgs.callPackage ./pkgs/{repo}/{mm(major, minor)}.nix {{ }};"

    text = DEFAULT_NIX.read_text()
    lines = text.splitlines()

    try:
        begin = next(i for i, l in enumerate(lines) if BEGIN_MARKER in l)
        end = next(i for i, l in enumerate(lines) if END_MARKER in l)
    except StopIteration:
        raise RuntimeError("default.nix is missing the auto-patcher marker block")

    block = lines[begin + 1 : end]
    preamble: list[str] = []  # non-entry lines (comments) kept verbatim, in order
    entries: dict[str, str] = {}
    for l in block:
        m = ENTRY_RE.match(l)
        if m:
            entries[m.group("attr")] = l.rstrip()
        elif l.strip():
            preamble.append(l.rstrip())

    entries[attr] = line  # set or replace
    new_block = preamble + [entries[a] for a in sorted(entries)]

    new_lines = lines[: begin + 1] + new_block + lines[end:]
    new_text = "\n".join(new_lines) + "\n"
    if new_text != text:
        DEFAULT_NIX.write_text(new_text)
        return True
    return False


# --------------------------------------------------------------------------- #
# commands
# --------------------------------------------------------------------------- #
def cmd_set(args: argparse.Namespace) -> dict:
    owner, repo, tag = args.owner, args.repo, args.tag
    version, major, minor = parse_version(tag)
    sri = args.hash or prefetch_hash(owner, repo, tag)

    ensure_build_recipe(owner, repo)
    vfile = write_version_file(owner, repo, major, minor, version, tag, sri)
    changed = update_default_nix(repo, major, minor)

    result = {
        "owner": owner,
        "repo": repo,
        "tag": tag,
        "version": version,
        "attr": attr_name(repo, major, minor),
        "file": str(vfile.relative_to(REPO_ROOT)),
        "hash": sri,
        "default_nix_changed": changed,
    }
    return result


def gh_api(path: str) -> list | dict:
    """Call the GitHub REST API via the preinstalled `gh` CLI (uses its auth)."""
    out = subprocess.run(
        ["gh", "api", "--paginate", path],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    # --paginate concatenates JSON arrays as separate documents; join them.
    docs = [json.loads(chunk) for chunk in _split_json(out)]
    if docs and isinstance(docs[0], list):
        merged: list = []
        for d in docs:
            merged.extend(d)
        return merged
    return docs[0] if docs else []


def _split_json(text: str) -> list[str]:
    """Split concatenated top-level JSON values produced by `gh --paginate`."""
    decoder = json.JSONDecoder()
    idx, n, out = 0, len(text), []
    while idx < n:
        while idx < n and text[idx].isspace():
            idx += 1
        if idx >= n:
            break
        _, end = decoder.raw_decode(text, idx)
        out.append(text[idx:end])
        idx = end
    return out


def latest_tag(owner: str, repo: str) -> str | None:
    tags = gh_api(f"repos/{owner}/{repo}/tags")
    for t in tags:
        name = t.get("name", "")
        try:
            parse_version(name)
        except ValueError:
            continue
        return name  # tags come newest-first from the API
    return None


def cmd_sync_all(args: argparse.Namespace) -> dict:
    owner = args.owner
    repos = gh_api(f"orgs/{owner}/repos")
    updated, skipped = [], []
    for r in repos:
        name = r.get("name", "")
        if name == args.self_repo or r.get("archived"):
            continue
        tag = latest_tag(owner, name)
        if tag is None:
            skipped.append({"repo": name, "reason": "no numeric tag"})
            continue
        res = cmd_set(
            argparse.Namespace(owner=owner, repo=name, tag=tag, hash=None)
        )
        updated.append(res)
    return {"updated": updated, "skipped": skipped}


# --------------------------------------------------------------------------- #
# cli
# --------------------------------------------------------------------------- #
def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="command", required=True)

    s = sub.add_parser("set", help="update a single package to a tag")
    s.add_argument("--owner", default=DEFAULT_OWNER)
    s.add_argument("--repo", required=True)
    s.add_argument("--tag", required=True)
    s.add_argument("--hash", help="SRI hash; skips nix prefetch (for tests)")
    s.set_defaults(func=cmd_set)

    a = sub.add_parser("sync-all", help="update every org repo to its latest tag")
    a.add_argument("--owner", default=DEFAULT_OWNER)
    a.add_argument("--self-repo", default="nur-packages")
    a.set_defaults(func=cmd_sync_all)

    args = p.parse_args(argv)
    result = args.func(args)
    json.dump(result, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
