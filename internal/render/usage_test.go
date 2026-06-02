package render

import (
	"testing"
	"time"
)

func TestAbbrevNum(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1.0k"},
		{4900, "4.9k"},
		{9999, "10.0k"},
		{10000, "10k"},
		{39000, "39k"},
		{999999, "1000k"},
		{1000000, "1.0M"},
		{1200000, "1.2M"},
	}
	for _, tc := range cases {
		got := abbrevNum(tc.n)
		if got != tc.want {
			t.Errorf("abbrevNum(%d): got %q want %q", tc.n, got, tc.want)
		}
	}
}

func TestAbbrevDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-1 * time.Second, "now"},
		{0, "now"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h"},
		{4 * time.Hour, "4h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{27 * 24 * time.Hour, "27d"},
	}
	for _, tc := range cases {
		got := abbrevDur(tc.d)
		if got != tc.want {
			t.Errorf("abbrevDur(%v): got %q want %q", tc.d, got, tc.want)
		}
	}
}
