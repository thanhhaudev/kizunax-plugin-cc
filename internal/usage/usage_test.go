package usage

import (
	"testing"
	"time"
)

func TestQuota_ZeroValueOK(t *testing.T) {
	var q Quota
	if q.Used != 0 || q.Limit != 0 || q.Remaining != 0 {
		t.Errorf("zero Quota not all zero")
	}
	if !q.ResetAt.IsZero() {
		t.Errorf("zero ResetAt should be zero time")
	}
}

func TestKeyUsage_NilQuotaPointers(t *testing.T) {
	k := KeyUsage{KeyMask: "kx_AbCd…"}
	if k.Coding != nil || k.Credits != nil {
		t.Errorf("nil pointers expected")
	}
	if k.AuthFailed {
		t.Errorf("AuthFailed zero should be false")
	}
}

func TestSnapshot_HoldsUsages(t *testing.T) {
	s := Snapshot{
		Provider: "openai",
		Usages: []KeyUsage{
			{KeyMask: "kx_AAAA…", FetchedAt: time.Now()},
			{KeyMask: "kx_BBBB…", AuthFailed: true},
		},
	}
	if len(s.Usages) != 2 {
		t.Fatalf("len: got %d", len(s.Usages))
	}
}
