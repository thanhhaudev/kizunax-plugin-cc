//go:build !windows

package cli

import (
	"testing"

	"github.com/thanhhaudev/llmreviewkit/git"
)

func TestBuildWorkerArgs(t *testing.T) {
	target := git.Target{Kind: git.TargetBranchDiff, Base: "master"}

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "strips strategy and paths, appends json",
			in:   []string{"--strategy=fanout", "--base", "master", "--paths", "api/", "--verbose"},
			want: []string{"--base", "master", "--verbose", "--json"},
		},
		{
			name: "strips strategy separate form",
			in:   []string{"--strategy", "auto", "--base", "master"},
			want: []string{"--base", "master", "--json"},
		},
		{
			name: "no strategy or paths - just appends json",
			in:   []string{"--base", "master", "--quiet"},
			want: []string{"--base", "master", "--quiet", "--json"},
		},
		{
			name: "already has json - adds another (idempotency not guaranteed, but works)",
			in:   []string{"--base", "master"},
			want: []string{"--base", "master", "--json"},
		},
		{
			name: "empty args",
			in:   nil,
			want: []string{"--json"},
		},
		{
			name: "strips paths= form",
			in:   []string{"--paths=app/Http", "--base", "master"},
			want: []string{"--base", "master", "--json"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildWorkerArgs(tc.in, target)
			if !sliceEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
