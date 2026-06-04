package bundlelog

import "testing"

func TestEnabled_RespectsEnvVar(t *testing.T) {
	t.Setenv("KIZUNAX_BUNDLE_LOG", "")
	if Enabled() {
		t.Fatalf("Enabled() must be false when env is empty")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "1")
	if !Enabled() {
		t.Fatalf("Enabled() must be true when env=1")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "true")
	if !Enabled() {
		t.Fatalf("Enabled() must be true when env=true")
	}

	t.Setenv("KIZUNAX_BUNDLE_LOG", "0")
	if Enabled() {
		t.Fatalf("Enabled() must be false when env=0")
	}
}
