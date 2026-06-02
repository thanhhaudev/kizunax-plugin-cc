package usage

import "testing"

func TestIsLow_NilNotLow(t *testing.T) {
	if IsLow(nil) {
		t.Errorf("nil quota should not be low")
	}
}

func TestIsLow_UnlimitedNeverLow(t *testing.T) {
	q := &Quota{Unlimited: true, Used: 999, Limit: 1000, Remaining: 1}
	if IsLow(q) {
		t.Errorf("unlimited should not be low")
	}
}

func TestIsLow_WithError(t *testing.T) {
	q := &Quota{Err: "timeout"}
	if IsLow(q) {
		t.Errorf("erred quota should not be low")
	}
}

func TestIsLow_LimitZeroDefensive(t *testing.T) {
	q := &Quota{Kind: "coding", Used: 0, Limit: 0, Remaining: 0}
	if IsLow(q) {
		t.Errorf("limit=0 should be defensive false")
	}
}

func TestIsLow_CodingPercentBoundary(t *testing.T) {
	// 20% remaining = NOT low (boundary)
	q := &Quota{Kind: "coding", Used: 80, Limit: 100, Remaining: 20}
	if IsLow(q) {
		t.Errorf("exactly 20%% remaining should not be low")
	}
	// 19% remaining = low
	q = &Quota{Kind: "coding", Used: 81, Limit: 100, Remaining: 19}
	if !IsLow(q) {
		t.Errorf("19%% remaining should be low")
	}
}

func TestIsLow_CodingAbsoluteFloor(t *testing.T) {
	// 4 absolute = low even if percent > 20%
	q := &Quota{Kind: "coding", Used: 96, Limit: 100, Remaining: 4}
	if !IsLow(q) {
		t.Errorf("4 remaining should be low (absolute floor)")
	}
	// 5 absolute = boundary, not low by floor; use Limit=25 so percent=20% (boundary, not low).
	q = &Quota{Kind: "coding", Used: 20, Limit: 25, Remaining: 5}
	if IsLow(q) {
		t.Errorf("5 remaining should not be low (absolute boundary — floor is < 5, not <= 5)")
	}
}

func TestIsLow_CreditsNoAbsoluteFloor(t *testing.T) {
	// 4 absolute on credits: percent = 4/100000 = 0.004% which IS < 20%, so it IS low.
	// This test asserts the absolute floor does NOT apply (would have been the same answer anyway),
	// so to exercise the no-absolute-floor branch, set Remaining=4 with Limit=10 (percent 40%, not low).
	q := &Quota{Kind: "credits", Used: 6, Limit: 10, Remaining: 4}
	if IsLow(q) {
		t.Errorf("credits at 4 absolute with 40%% remaining should NOT be low (absolute floor must not apply)")
	}
	// Credits at 25% = not low
	q = &Quota{Kind: "credits", Used: 75000, Limit: 100000, Remaining: 25000}
	if IsLow(q) {
		t.Errorf("credits at 25%% should not be low")
	}
}

func TestKeyIsLow(t *testing.T) {
	low := &Quota{Kind: "coding", Used: 96, Limit: 100, Remaining: 4}
	high := &Quota{Kind: "credits", Used: 1000, Limit: 100000, Remaining: 99000}

	if !KeyIsLow(KeyUsage{Coding: low, Credits: high}) {
		t.Errorf("coding low should mark key low")
	}
	if !KeyIsLow(KeyUsage{Coding: high, Credits: low}) {
		t.Errorf("credits low should mark key low")
	}
	if KeyIsLow(KeyUsage{Coding: high, Credits: high}) {
		t.Errorf("both high should not be low")
	}
	if KeyIsLow(KeyUsage{}) {
		t.Errorf("empty KeyUsage should not be low")
	}
}
