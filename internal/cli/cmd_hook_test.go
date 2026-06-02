package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func TestSessionCleanup_OnStart_WritesEnvFile(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "env.sh")

	// Simulate Claude Code stdin payload for SessionStart event.
	stdin := strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"sess-xyz","cwd":"` + tmp + `"}`)
	t.Setenv("CLAUDE_ENV_FILE", envFile)

	ws := state.WorkspaceDir{Root: t.TempDir()}

	var stdout, stderr bytes.Buffer
	runHookSessionCleanup(ws, stdin, &stdout, &stderr)

	data, _ := os.ReadFile(envFile)
	if !strings.Contains(string(data), "KIZUNAX_SESSION_ID='sess-xyz'") {
		t.Errorf("env file missing session id; got: %s", data)
	}
}

func TestSessionCleanup_OnEnd_DoesNotWriteEnvFile(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "env.sh")
	t.Setenv("CLAUDE_ENV_FILE", envFile)

	ws := state.WorkspaceDir{Root: t.TempDir()}
	if err := os.MkdirAll(ws.JobsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	stdin := strings.NewReader(`{"hook_event_name":"SessionEnd","session_id":"sess-end","cwd":"` + tmp + `"}`)
	var stdout, stderr bytes.Buffer
	runHookSessionCleanup(ws, stdin, &stdout, &stderr)

	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Errorf("env file should not be created on SessionEnd; stat err: %v", err)
	}
}
