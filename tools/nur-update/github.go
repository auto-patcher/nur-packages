package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
)

// ghList calls a paginated GitHub REST endpoint via the `gh` CLI (using its
// own auth) and decodes the concatenated pages into a flat slice.
func ghList[T any](path string) ([]T, error) {
	cmd := exec.Command("gh", "api", "--paginate", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh api %s: %w: %s", path, err, stderr.String())
	}
	// `gh --paginate` concatenates one JSON array per page; decode them in turn.
	dec := json.NewDecoder(&stdout)
	var all []T
	for {
		var page []T
		if err := dec.Decode(&page); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		all = append(all, page...)
	}
	return all, nil
}

type repoInfo struct {
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type tagInfo struct {
	Name string `json:"name"`
}

// latestTag returns the newest tag whose name carries a numeric version, or ""
// if the repo has none. The API returns tags newest-first.
func latestTag(owner, repo string) (string, error) {
	tags, err := ghList[tagInfo](fmt.Sprintf("repos/%s/%s/tags", owner, repo))
	if err != nil {
		return "", err
	}
	for _, t := range tags {
		if _, err := parseVersion(t.Name); err == nil {
			return t.Name, nil
		}
	}
	return "", nil
}

type syncSummary struct {
	Updated []result            `json:"updated"`
	Skipped []map[string]string `json:"skipped"`
}

func runSyncAll(args []string) error {
	fs := flag.NewFlagSet("sync-all", flag.ExitOnError)
	owner := fs.String("owner", defaultOwner, "org to enumerate")
	selfRepo := fs.String("self-repo", "nur-packages", "this repo, skipped")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repos, err := ghList[repoInfo](fmt.Sprintf("orgs/%s/repos", *owner))
	if err != nil {
		return err
	}

	var sum syncSummary
	for _, r := range repos {
		if r.Name == *selfRepo || r.Archived {
			continue
		}
		tag, err := latestTag(*owner, r.Name)
		if err != nil {
			return err
		}
		if tag == "" {
			sum.Skipped = append(sum.Skipped, map[string]string{"repo": r.Name, "reason": "no numeric tag"})
			continue
		}
		res, err := setPackage(*owner, r.Name, tag, "")
		if err != nil {
			return err
		}
		sum.Updated = append(sum.Updated, res)
	}
	return printJSON(sum)
}
