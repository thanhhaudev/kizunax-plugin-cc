package glossary

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxGlossaryBytes = 16 * 1024

type Glossary struct {
	Path      string
	Content   string
	Truncated bool
}

var candidatePaths = []string{
	filepath.Join(".kizunax", "glossary.md"),
	filepath.Join("docs", "glossary.md"),
	"GLOSSARY.md",
}

// Load searches workspaceRoot for a glossary file in priority order
// (.kizunax/glossary.md > docs/glossary.md > GLOSSARY.md) and returns
// the verbatim content capped at 16 KiB. No file found → empty Glossary.
// Content is treated verbatim; no parsing.
func Load(workspaceRoot string) (Glossary, error) {
	for _, rel := range candidatePaths {
		abs := filepath.Join(workspaceRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Glossary{}, fmt.Errorf("stat %s: %w", abs, err)
		}
		if info.IsDir() {
			continue
		}
		f, err := os.Open(abs)
		if err != nil {
			return Glossary{}, fmt.Errorf("open %s: %w", abs, err)
		}
		defer f.Close()

		buf, err := io.ReadAll(io.LimitReader(f, int64(maxGlossaryBytes)+1))
		if err != nil {
			return Glossary{}, fmt.Errorf("read %s: %w", abs, err)
		}
		truncated := len(buf) > maxGlossaryBytes
		if truncated {
			buf = buf[:maxGlossaryBytes]
		}
		return Glossary{
			Path:      abs,
			Content:   string(buf),
			Truncated: truncated,
		}, nil
	}
	return Glossary{}, nil
}
