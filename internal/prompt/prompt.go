package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
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

// Build assembles the user prompt by interpolating the chosen template
// with target label, schema, diff bundle, and optional focus text.
func Build(pluginRoot string, mode Mode, bundle diff.Bundle, schemaJSON, focus string) (Prompt, error) {
	tmplPath := filepath.Join(pluginRoot, "prompts", mode.TemplateFile())
	raw, err := os.ReadFile(tmplPath)
	if err != nil {
		return Prompt{}, xerrors.Internal("load_template", fmt.Sprintf("cannot read %s", tmplPath), err)
	}

	user := interpolate(string(raw), map[string]string{
		"TARGET_LABEL":  bundle.TargetLabel,
		"SCHEMA_INLINE": schemaJSON,
		"REVIEW_INPUT":  diff.RenderForPrompt(bundle),
		"USER_FOCUS":    formatFocus(focus),
	})

	return Prompt{System: defaultSystem, User: user}, nil
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
