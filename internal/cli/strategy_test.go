package cli

import (
	"testing"
)

func TestFilterStrategyFlag(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"no strategy", []string{"--base", "master"}, []string{"--base", "master"}},
		{"separate", []string{"--strategy", "auto", "--base", "master"}, []string{"--base", "master"}},
		{"equals", []string{"--strategy=fanout", "--base", "master"}, []string{"--base", "master"}},
		{"mixed", []string{"--verbose", "--strategy=single", "--paths", "api/", "--base", "master"}, []string{"--verbose", "--paths", "api/", "--base", "master"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterStrategyFlag(tc.in)
			if !sliceEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
