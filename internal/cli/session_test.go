package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSessionEnv_AppendsExport(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "env.sh")
	if err := os.WriteFile(envFile, []byte("# existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteSessionEnv(envFile, "sess-abc123"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(envFile)
	want := "export KIZUNAX_SESSION_ID='sess-abc123'"
	if !strings.Contains(string(data), want) {
		t.Errorf("env file missing %q; got: %s", want, data)
	}
	if !strings.HasPrefix(string(data), "# existing\n") {
		t.Errorf("WriteSessionEnv overwrote existing content: %s", data)
	}
}

func TestWriteSessionEnv_EmptyFileOrSessionIsNoop(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "env.sh")
	if err := WriteSessionEnv("", "sess"); err != nil {
		t.Errorf("empty envFile should be silent no-op, got: %v", err)
	}
	if err := WriteSessionEnv(envFile, ""); err != nil {
		t.Errorf("empty session should be silent no-op, got: %v", err)
	}
	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Errorf("no-op should not create file")
	}
}

func TestCurrentSessionID_ReadsEnv(t *testing.T) {
	t.Setenv("KIZUNAX_SESSION_ID", "sess-zzz")
	if got := CurrentSessionID(); got != "sess-zzz" {
		t.Errorf("got %q, want sess-zzz", got)
	}
}
