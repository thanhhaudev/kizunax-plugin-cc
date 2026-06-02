package usage

const (
	lowPercentThreshold = 0.20
	codingAbsoluteFloor = 5 // applies only to request-shaped Coding quota
)

// IsLow reports whether a single quota is "low" by the v0.8 rules:
//   - nil / Unlimited / errored / Limit<=0 → never low
//   - Coding with Remaining < 5 → low (absolute floor)
//   - any kind with Remaining/Limit < 20% → low
func IsLow(q *Quota) bool {
	if q == nil || q.Unlimited || q.Err != "" {
		return false
	}
	if q.Limit <= 0 {
		return false
	}
	if q.Kind == "coding" && q.Remaining < codingAbsoluteFloor {
		return true
	}
	return float64(q.Remaining)/float64(q.Limit) < lowPercentThreshold
}

// KeyIsLow returns true when either of a key's quotas is IsLow.
func KeyIsLow(k KeyUsage) bool {
	return IsLow(k.Coding) || IsLow(k.Credits)
}
