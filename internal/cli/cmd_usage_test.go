package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

func TestRenderUsageOutput_TwoKeys(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	usages := []usage.KeyUsage{
		{
			KeyMask: "kx_AAA…",
			Coding:  &usage.Quota{Kind: "coding", Plan: "pro", Used: 1000, Limit: 50000, Remaining: 49000, ResetAt: now.Add(20 * time.Minute)},
			Credits: &usage.Quota{Kind: "credits", Plan: "pro", Used: 100, Limit: 100000, Remaining: 99900, ResetAt: now.Add(27 * 24 * time.Hour)},
		},
		{KeyMask: "kx_BBB…", AuthFailed: true},
	}
	var buf bytes.Buffer
	writeUsageOutput(&buf, "openai", "round-robin", usages, now)
	got := buf.String()
	if !strings.Contains(got, "[1] kx_AAA…") || !strings.Contains(got, "[2] kx_BBB…") {
		t.Errorf("rows missing in output:\n%s", got)
	}
	if !strings.Contains(got, "auth failed") {
		t.Errorf("auth fail row missing:\n%s", got)
	}
	if !strings.Contains(got, "Provider: openai") {
		t.Errorf("provider header missing:\n%s", got)
	}
}
