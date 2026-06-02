package render

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/usage"
)

// abbrevNum compresses an int64 to a short human-readable form.
//
//	<1000        → raw integer ("950")
//	<10_000      → one-decimal k ("4.9k")
//	<1_000_000   → integer k ("39k")
//	≥1_000_000   → one-decimal M ("1.2M")
func abbrevNum(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}

// abbrevDur compresses a Duration to ≤6 visible chars.
//
//	d ≤ 0   → "now"
//	<60s    → "Xs"
//	<60m    → "Xm"
//	<24h    → "Xh"
//	≥24h    → "Xd"
func abbrevDur(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours())/24)
}

const (
	usageBarWidth = 20
	barFilled     = "█"
	barEmpty      = "░"
)

// RenderUsage formats a Snapshot to markdown using time.Now() as the "now"
// for relative-reset rendering. Exposes RenderUsageAt for deterministic
// golden tests.
func RenderUsage(s usage.Snapshot, rotation string) string {
	return RenderUsageAt(s, rotation, time.Now())
}

// RenderUsageAt is RenderUsage with an explicit "now" anchor (for tests).
func RenderUsageAt(s usage.Snapshot, rotation string, now time.Time) string {
	var b strings.Builder
	if rotation == "" {
		rotation = "round-robin"
	}
	fmt.Fprintf(&b, "Key Pool — %d keys, rotation: %s\n\n", len(s.Usages), rotation)

	for i, ku := range s.Usages {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, ku.KeyMask)
		if ku.AuthFailed {
			b.WriteString("    — auth failed (HTTP 401)\n\n")
			continue
		}
		writeQuotaLine(&b, "Coding ", ku.Coding, now)
		writeQuotaLine(&b, "Credits", ku.Credits, now)
		b.WriteString("\n")
	}
	return b.String()
}

func writeQuotaLine(b *strings.Builder, label string, q *usage.Quota, now time.Time) {
	if q == nil {
		fmt.Fprintf(b, "    %s  — not fetched\n", label)
		return
	}
	if q.Err != "" {
		fmt.Fprintf(b, "    %s  — unavailable (%s)\n", label, q.Err)
		return
	}
	plan := fmt.Sprintf("(%s)", q.Plan)
	if q.Unlimited {
		bar := strings.Repeat(barFilled, usageBarWidth)
		fmt.Fprintf(b, "    %s  %-13s %s  ∞  unlimited\n", label, plan, bar)
		return
	}
	pct := 0.0
	if q.Limit > 0 {
		pct = float64(q.Used) / float64(q.Limit) * 100
	}
	bar := makeBar(pct)
	tokensSuffix := ""
	if q.Kind == "credits" {
		tokensSuffix = " tok"
	}
	used := abbrevNum(q.Used)
	limit := abbrevNum(q.Limit)
	reset := abbrevDur(q.ResetAt.Sub(now))
	low := ""
	if usage.IsLow(q) {
		low = "  ⚠️ LOW"
	}
	fmt.Fprintf(b, "    %s  %-13s %s  %3d%%  %s / %s%s    %s%s\n",
		label, plan, bar, int(math.Round(pct)), used, limit, tokensSuffix, reset, low)
}

func makeBar(percentUsed float64) string {
	if percentUsed < 0 {
		percentUsed = 0
	}
	if percentUsed > 100 {
		percentUsed = 100
	}
	filled := int(math.Round(percentUsed / 100 * float64(usageBarWidth)))
	if filled < 0 {
		filled = 0
	}
	if filled > usageBarWidth {
		filled = usageBarWidth
	}
	return strings.Repeat(barFilled, filled) + strings.Repeat(barEmpty, usageBarWidth-filled)
}
