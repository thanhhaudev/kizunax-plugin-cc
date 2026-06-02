package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionCleanup_OnStart_WritesEnvFile(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "env.sh")

	// Simulate Claude Code stdin payload for SessionStart event.
	stdin := strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"sess-xyz","cwd":"` + tmp + `"}`)
	t.Setenv("CLAUDE_ENV_FILE", envFile)

	var stdout, stderr bytes.Buffer
	if err := runHookSessionCleanup(stdin, &stdout, &stderr); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}

	data, _ := os.ReadFile(envFile)
	if !strings.Contains(string(data), "KIZUNAX_SESSION_ID='sess-xyz'") {
		t.Errorf("env file missing session id; got: %s", data)
	}
}
