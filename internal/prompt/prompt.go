package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/diff"
	xerrors "github.com/thanhhaudev/kizunax-plugin-cc/internal/errors"
)

type Prompt struct {
	System string
	User   string
}

const defaultSystem = "You are a senior code reviewer. Output ONLY valid JSON matching the schema provided in the user message. No prose, no code fences, no commentary outside the JSON object."

func Build(pluginRoot string, bundle diff.Bundle, schemaJSON string) (Prompt, error) {
	tmplPath := filepath.Join(pluginRoot, "prompts", "review.md")
	raw, err := os.ReadFile(tmplPath)
	if err != nil {
		return Prompt{}, xerrors.Internal("load_template", fmt.Sprintf("cannot read %s", tmplPath), err)
	}

	user := interpolate(string(raw), map[string]string{
		"TARGET_LABEL":  bundle.TargetLabel,
		"SCHEMA_INLINE": schemaJSON,
		"REVIEW_INPUT":  diff.RenderForPrompt(bundle),
	})

	return Prompt{System: defaultSystem, User: user}, nil
}

func interpolate(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
