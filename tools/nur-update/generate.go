package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.ParseFS(templatesFS, "templates/*.tmpl"))

const defaultOwner = "auto-patcher"

var (
	// A generated per-version file: e.g. 1_2.nix. build.nix is excluded.
	versionFileRe = regexp.MustCompile(`^[0-9]+_[0-9]+\.nix$`)
	// A tag's leading numeric version: v1.2.3 / 1.2 / v2 ...
	tagRe = regexp.MustCompile(`^[vV]?([0-9]+)(?:\.([0-9]+))?(?:\.([0-9]+))?`)
)

func repoRoot() string {
	if r := os.Getenv("NUR_REPO_ROOT"); r != "" {
		return r
	}
	return "."
}

func pkgsDir() string        { return filepath.Join(repoRoot(), "pkgs") }
func defaultNixPath() string { return filepath.Join(repoRoot(), "default.nix") }

// --------------------------------------------------------------------------- //
// version / naming
// --------------------------------------------------------------------------- //

type version struct {
	Full  string // e.g. "1.2.3"
	Major int
	Minor int
}

func (v version) mm() string { return fmt.Sprintf("%d_%d", v.Major, v.Minor) }

func parseVersion(tag string) (version, error) {
	m := tagRe.FindStringSubmatch(strings.TrimSpace(tag))
	if m == nil {
		return version{}, fmt.Errorf("tag %q does not start with a numeric version", tag)
	}
	major, _ := strconv.Atoi(m[1])
	minor := 0
	if m[2] != "" {
		minor, _ = strconv.Atoi(m[2])
	}
	var nums []string
	for _, g := range m[1:] {
		if g != "" {
			nums = append(nums, g)
		}
	}
	return version{Full: strings.Join(nums, "."), Major: major, Minor: minor}, nil
}

// --------------------------------------------------------------------------- //
// template rendering
// --------------------------------------------------------------------------- //

func render(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// ensureBuildRecipe writes pkgs/<repo>/build.nix from the template if absent.
// It never overwrites an existing recipe.
func ensureBuildRecipe(owner, repo string) error {
	path := filepath.Join(pkgsDir(), repo, "build.nix")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	out, err := render("build.nix.tmpl", map[string]string{"Owner": owner, "Repo": repo})
	if err != nil {
		return err
	}
	return writeFile(path, out)
}

func writeVersionFile(owner, repo, tag string, v version, sri string) (string, error) {
	path := filepath.Join(pkgsDir(), repo, v.mm()+".nix")
	out, err := render("version.nix.tmpl", map[string]string{
		"Owner":   owner,
		"Repo":    repo,
		"Tag":     tag,
		"Version": v.Full,
		"Rev":     tag,
		"Hash":    sri,
	})
	if err != nil {
		return "", err
	}
	return path, writeFile(path, out)
}

// --------------------------------------------------------------------------- //
// default.nix regeneration (filesystem is the source of truth)
// --------------------------------------------------------------------------- //

// nixEntry is one line in default.nix: `Attr = pkgs.callPackage Path { };`.
type nixEntry struct {
	Attr string // e.g. widget_1_2
	Path string // e.g. ./pkgs/widget/1_2.nix
}

// metaPackages are the NUR's own tooling, declared statically (not produced by
// the auto-patcher loop). They live as their own derivations under pkgs/ and
// show up in the namespace alongside the auto-patched packages.
var metaPackages = []nixEntry{
	{Attr: "nur-update", Path: "./pkgs/nur-update"},
}

// scanPackages discovers the auto-patched, versioned packages by walking the
// pkgs/ tree for <major>_<minor>.nix files (build.nix and meta dirs ignored).
func scanPackages() ([]nixEntry, error) {
	dirs, err := os.ReadDir(pkgsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []nixEntry
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(pkgsDir(), d.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() || !versionFileRe.MatchString(f.Name()) {
				continue
			}
			base := strings.TrimSuffix(f.Name(), ".nix")
			entries = append(entries, nixEntry{
				Attr: d.Name() + "_" + base,
				Path: fmt.Sprintf("./pkgs/%s/%s.nix", d.Name(), base),
			})
		}
	}
	return entries, nil
}

// allEntries merges the static meta packages with the scanned ones, sorted by
// attr so default.nix output is stable.
func allEntries() ([]nixEntry, error) {
	scanned, err := scanPackages()
	if err != nil {
		return nil, err
	}
	entries := append(append([]nixEntry{}, metaPackages...), scanned...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Attr < entries[j].Attr })
	return entries, nil
}

func regenDefaultNix() ([]nixEntry, error) {
	entries, err := allEntries()
	if err != nil {
		return nil, err
	}
	out, err := render("default.nix.tmpl", map[string]any{"Packages": entries})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(defaultNixPath(), out, 0o644); err != nil {
		return nil, err
	}
	return entries, nil
}

// --------------------------------------------------------------------------- //
// hashing
// --------------------------------------------------------------------------- //

func archiveURL(owner, repo, tag string) string {
	return fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.tar.gz", owner, repo, tag)
}

// prefetchHash returns the SRI hash of the unpacked tag tarball, matching the
// unpacked-NAR hash fetchFromGitHub expects.
func prefetchHash(owner, repo, tag string) (string, error) {
	base32, err := run("nix-prefetch-url", "--unpack", "--type", "sha256", archiveURL(owner, repo, tag))
	if err != nil {
		return "", fmt.Errorf("nix-prefetch-url: %w", err)
	}
	base32 = strings.TrimSpace(base32)
	// Newer nix: `nix hash convert`; older: `nix hash to-sri`.
	for _, args := range [][]string{
		{"hash", "convert", "--hash-algo", "sha256", "--to", "sri", base32},
		{"hash", "to-sri", "--type", "sha256", base32},
	} {
		if sri, err := run("nix", args...); err == nil {
			return strings.TrimSpace(sri), nil
		}
	}
	return "", fmt.Errorf("could not convert %q to SRI", base32)
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// --------------------------------------------------------------------------- //
// the core operation
// --------------------------------------------------------------------------- //

type result struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Tag     string `json:"tag"`
	Version string `json:"version"`
	Attr    string `json:"attr"`
	File    string `json:"file"`
	Hash    string `json:"hash"`
}

// setPackage stamps a package version and regenerates default.nix. If hash is
// empty it is prefetched via nix.
func setPackage(owner, repo, tag, hash string) (result, error) {
	v, err := parseVersion(tag)
	if err != nil {
		return result{}, err
	}
	if hash == "" {
		hash, err = prefetchHash(owner, repo, tag)
		if err != nil {
			return result{}, err
		}
	}
	if err := ensureBuildRecipe(owner, repo); err != nil {
		return result{}, err
	}
	path, err := writeVersionFile(owner, repo, tag, v, hash)
	if err != nil {
		return result{}, err
	}
	if _, err := regenDefaultNix(); err != nil {
		return result{}, err
	}
	rel, _ := filepath.Rel(repoRoot(), path)
	return result{
		Owner:   owner,
		Repo:    repo,
		Tag:     tag,
		Version: v.Full,
		Attr:    repo + "_" + v.mm(),
		File:    rel,
		Hash:    hash,
	}, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --------------------------------------------------------------------------- //
// commands
// --------------------------------------------------------------------------- //

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	owner := fs.String("owner", defaultOwner, "org that owns the source repo")
	repo := fs.String("repo", "", "source repo name (required)")
	tag := fs.String("tag", "", "tag to package, e.g. v1.2.3 (required)")
	hash := fs.String("hash", "", "SRI hash; skips nix prefetch (for tests)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" || *tag == "" {
		return fmt.Errorf("--repo and --tag are required")
	}
	res, err := setPackage(*owner, *repo, *tag, *hash)
	if err != nil {
		return err
	}
	return printJSON(res)
}

func runRegen(args []string) error {
	fs := flag.NewFlagSet("regen", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	entries, err := regenDefaultNix()
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"packages": entries})
}
