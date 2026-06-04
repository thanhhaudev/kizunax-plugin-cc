package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/pkg/errors"
)

type Mode int

const (
	ModeStandard Mode = iota + 1
	ModeAdversarial
)

func (m Mode) TemplateFile() string {
	switch m {
	case ModeAdversarial:
		return "adversarial-review.md"
	default:
		return "review.md"
	}
}

func (m Mode) String() string {
	switch m {
	case ModeAdversarial:
		return "adversarial"
	default:
		return "standard"
	}
}

type Prompt struct {
	System string
	User   string
}

const defaultSystem = "You are a senior code reviewer. Output ONLY valid JSON matching the schema provided in the user message. No prose, no code fences, no commentary outside the JSON object."

const glossarySectionTemplate = "## Project glossary\n\n%s\n\n---\n\n%s"

// Build assembles the user prompt by interpolating the chosen template
// with target label, schema, diff bundle, optional focus text, and optional glossary.
// When glossary is non-empty it is prepended to the system prompt.
func Build(pluginRoot string, mode Mode, bundle diff.Bundle, schemaJSON, focus, glossary string) (Prompt, error) {
	tmplPath := filepath.Join(pluginRoot, "prompts", mode.TemplateFile())
	raw, err := os.ReadFile(tmplPath)
	if err != nil {
		return Prompt{}, xerrors.Internal("load_template", fmt.Sprintf("cannot read %s", tmplPath), err)
	}

	user := interpolate(string(raw), map[string]string{
		"TARGET_LABEL":     bundle.TargetLabel,
		"SCHEMA_INLINE":    schemaJSON,
		"REVIEW_INPUT":     diff.RenderForPrompt(bundle),
		"USER_FOCUS":       formatFocus(focus),
		"REFERENCED_FILES": renderReferencedFiles(bundle.ReferencedFiles),
	})

	system := defaultSystem
	if strings.TrimSpace(glossary) != "" {
		system = fmt.Sprintf(glossarySectionTemplate, glossary, defaultSystem)
	}

	return Prompt{System: system, User: user}, nil
}

func formatFocus(focus string) string {
	focus = strings.TrimSpace(focus)
	if focus == "" {
		return ""
	}
	return "User focus:\n" + focus + "\n"
}

func interpolate(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func renderReferencedFiles(files []diff.ReferencedFile) string {
	if len(files) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Referenced files for context (read-only)\n\n")
	sb.WriteString("These files contain definitions referenced by symbols in the diff.\n")
	sb.WriteString("Use them to understand types, constants, and helpers.\n")
	sb.WriteString("DO NOT flag findings in these files — they are unchanged context.\n\n")
	for _, f := range files {
		matched := ""
		if len(f.Symbols) > 0 {
			matched = " (matched: " + strings.Join(f.Symbols, ", ") + ")"
		}
		fmt.Fprintf(&sb, "### %s%s\n```\n%s\n```\n\n", f.Path, matched, f.Excerpt)
	}
	return strings.TrimRight(sb.String(), "\n")
}
