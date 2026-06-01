package cli

import "strings"

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
		if strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

// flagValue returns the value for --name <val> or --name=val. Empty if not present.
func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}

func splitPaths(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
