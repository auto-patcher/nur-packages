// Command auto-packager deterministically generates and updates the
// auto-patcher NUR package set from source-repo tags. No LLM in the loop: every
// Nix file is rendered from a Go text/template, and default.nix is derived by
// scanning the pkgs/ tree (the filesystem is the source of truth).
//
//	auto-packager set --repo <name> --tag <tag> [--owner o] [--hash sri]
//	auto-packager sync-all [--owner o] [--self-repo nur-packages]
//	auto-packager regen
//
// Paths are taken relative to the repo root, which defaults to the current
// directory and can be overridden with NUR_REPO_ROOT.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "set":
		err = runSet(args)
	case "sync-all":
		err = runSyncAll(args)
	case "regen":
		err = runRegen(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  auto-packager set --repo <name> --tag <tag> [--owner o] [--hash sri]
  auto-packager sync-all [--owner o] [--self-repo nur-packages]
  auto-packager regen
`)
}
