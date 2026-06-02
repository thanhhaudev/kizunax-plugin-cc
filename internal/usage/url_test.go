package usage

import "testing"

func TestUsageURL_StripsTrailingSlash(t *testing.T) {
	got := usageURL("https://kizunax.io/")
	want := "https://kizunax.io/api/coding/v1/usage"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestUsageURL_NoTrailingSlash(t *testing.T) {
	got := usageURL("https://kizunax.io")
	want := "https://kizunax.io/api/coding/v1/usage"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestQuotaURL(t *testing.T) {
	got := quotaURL("https://kizunax.io")
	want := "https://kizunax.io/api/v1/quota"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDeriveBase_FromConfigBaseURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://kizunax.io/api/coding/v1", "https://kizunax.io"},
		{"https://kizunax.io/api/coding/anthropic/v1", "https://kizunax.io"},
		{"http://127.0.0.1:8080/api/v1", "http://127.0.0.1:8080"},
	}
	for _, tc := range cases {
		got, err := DeriveBase(tc.in)
		if err != nil {
			t.Errorf("%s: err: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeriveBase_Invalid(t *testing.T) {
	if _, err := DeriveBase(""); err == nil {
		t.Errorf("empty base should error")
	}
	if _, err := DeriveBase("not a url"); err == nil {
		t.Errorf("malformed should error")
	}
	if _, err := DeriveBase("/relative/path"); err == nil {
		t.Errorf("missing host should error")
	}
}
