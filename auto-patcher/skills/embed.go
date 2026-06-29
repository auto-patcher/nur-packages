// Package skills embeds the autopatcher prompt templates.
// Each SKILL.md is a Go text/template rendered by dispatcher/internal/prompts.
package skills

import "embed"

//go:embed patch-dissect/SKILL.md
//go:embed patch-design/SKILL.md
//go:embed patch-apply/SKILL.md
//go:embed patch-init/SKILL.md
var FS embed.FS
