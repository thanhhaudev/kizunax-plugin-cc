package render

import (
	"fmt"
	"time"
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
