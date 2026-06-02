package cli

import (
	"fmt"
	"os"
	"strings"
)

// SessionIDEnv is the env var holding the current CC session ID.
// Set by `kizunax hook session-cleanup` on SessionStart via CLAUDE_ENV_FILE.
const SessionIDEnv = "KIZUNAX_SESSION_ID"

// WriteSessionEnv appends `export KIZUNAX_SESSION_ID='<id>'` to envFile.
// Silent no-op if envFile or sessionID is empty (e.g. running outside Claude Code).
func WriteSessionEnv(envFile, sessionID string) error {
	if envFile == "" || sessionID == "" {
		return nil
	}
	line := fmt.Sprintf("export %s=%s\n", SessionIDEnv, shellSingleQuote(sessionID))
	f, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open CLAUDE_ENV_FILE: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write CLAUDE_ENV_FILE: %w", err)
	}
	return nil
}

// CurrentSessionID returns the session ID set by the SessionStart hook,
// or empty string when running outside a CC session.
func CurrentSessionID() string {
	return os.Getenv(SessionIDEnv)
}

func shellSingleQuote(s string) string {
	// POSIX-safe: wrap in single quotes, escape any embedded single quote as '\''
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
