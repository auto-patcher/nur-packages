// Package prompts renders autopatcher SKILL.md templates into Claude prompts.
package prompts

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/auto-patcher/skills/skills"
)

// Context holds the per-repo values injected into prompt templates.
type Context struct {
	Repo        string // "owner/repo" of the fork
	Upstream    string // "owner/repo" of the upstream
	LastPatched string // last patched upstream tag, e.g. "v1.4.0"
}

// RenderCycle composes a full patch cycle prompt (dissect → design → apply)
// with shared context injected once at the top.
func RenderCycle(ctx Context) (string, error) {
	dissect, err := render("patch-dissect", ctx)
	if err != nil {
		return "", fmt.Errorf("render patch-dissect: %w", err)
	}
	design, err := render("patch-design", ctx)
	if err != nil {
		return "", fmt.Errorf("render patch-design: %w", err)
	}
	apply, err := render("patch-apply", ctx)
	if err != nil {
		return "", fmt.Errorf("render patch-apply: %w", err)
	}

	return fmt.Sprintf(`You are the autopatcher running a full patch cycle.

## Repository

- Fork:         %s
- Upstream:     %s
- Last patched: %s

The repository is cloned in your current working directory.
Read PATCHER.md before starting — it describes the fork's character,
architecture, and style. Work through each phase in sequence; complete
and verify each before moving to the next.

---

# Phase 1 — Dissect

%s

---

# Phase 2 — Design

%s

---

# Phase 3 — Apply

%s
`, ctx.Repo, ctx.Upstream, ctx.LastPatched, dissect, design, apply), nil
}

// RenderSingle renders a single skill prompt. Useful for re-running one phase.
func RenderSingle(skill string, ctx Context) (string, error) {
	body, err := render(skill, ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`You are the autopatcher.

## Repository

- Fork:         %s
- Upstream:     %s
- Last patched: %s

The repository is cloned in your current working directory.

---

%s
`, ctx.Repo, ctx.Upstream, ctx.LastPatched, body), nil
}

func render(name string, ctx Context) (string, error) {
	content, err := skills.FS.ReadFile(name + "/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("read %s/SKILL.md: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}
