package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestParseVersion(t *testing.T) {
	cases := []struct {
		tag          string
		full         string
		major, minor int
	}{
		{"v1.2.3", "1.2.3", 1, 2},
		{"1.2", "1.2", 1, 2},
		{"v2", "2", 2, 0},
		{"V3.4.5-rc1", "3.4.5", 3, 4},
	}
	for _, c := range cases {
		v, err := parseVersion(c.tag)
		if err != nil {
			t.Fatalf("parseVersion(%q) error: %v", c.tag, err)
		}
		if v.Full != c.full || v.Major != c.major || v.Minor != c.minor {
			t.Errorf("parseVersion(%q) = %+v, want full=%s major=%d minor=%d",
				c.tag, v, c.full, c.major, c.minor)
		}
	}
	for _, bad := range []string{"main", "release-x", ""} {
		if _, err := parseVersion(bad); err == nil {
			t.Errorf("parseVersion(%q) expected error", bad)
		}
	}
}

func TestSetPackageAndRegen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NUR_REPO_ROOT", root)

	// A pre-existing, hand-written example package (like the hello seeds): the
	// scan must pick it up even though setPackage never touched it.
	must(t, os.MkdirAll(filepath.Join(root, "pkgs", "hello"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "pkgs", "hello", "1_0.nix"), []byte("{}\n"), 0o644))

	mustSet(t, "widget", "v1.2.3")
	mustSet(t, "widget", "v1.2.9") // same major_minor -> overwrites the file/line
	mustSet(t, "alpha", "3.0")     // sorts before hello/widget

	out, err := os.ReadFile(filepath.Join(root, "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "pkgs.callPackage") {
			entries = append(entries, strings.TrimSpace(l))
		}
	}
	want := []string{
		"alpha_3_0 = pkgs.callPackage ./pkgs/alpha/3_0.nix { };",
		// auto-packager is a static meta package, always present and sorted in.
		"auto-packager = pkgs.callPackage ./pkgs/auto-packager { };",
		"hello_1_0 = pkgs.callPackage ./pkgs/hello/1_0.nix { };",
		"widget_1_2 = pkgs.callPackage ./pkgs/widget/1_2.nix { };",
	}
	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		t.Fatalf("default.nix entries =\n%s\nwant\n%s", strings.Join(entries, "\n"), strings.Join(want, "\n"))
	}

	// Latest version + hash landed in the (single) widget version file.
	vf, err := os.ReadFile(filepath.Join(root, "pkgs", "widget", "1_2.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(vf), `version = "1.2.9";`) || !strings.Contains(string(vf), fakeHash) {
		t.Fatalf("widget/1_2.nix missing version/hash:\n%s", vf)
	}

	// build.nix written once, contains the real recipe (not the thin file).
	bf, err := os.ReadFile(filepath.Join(root, "pkgs", "widget", "build.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bf), "stdenv.mkDerivation") {
		t.Fatalf("widget/build.nix not a build recipe:\n%s", bf)
	}

	// Idempotent: regen with no fs change must be a no-op (stable output).
	before, _ := os.ReadFile(filepath.Join(root, "default.nix"))
	if _, err := regenDefaultNix(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "default.nix"))
	if string(before) != string(after) {
		t.Fatal("regen not idempotent")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustSet(t *testing.T, repo, tag string) {
	t.Helper()
	if _, err := setPackage(defaultOwner, repo, tag, fakeHash); err != nil {
		t.Fatalf("setPackage(%s, %s): %v", repo, tag, err)
	}
}
