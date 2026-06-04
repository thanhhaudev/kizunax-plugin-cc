//go:build windows

package job

import (
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/kizunax-plugin-cc/internal/state"
)

func SpawnBackground(cwd string, ws state.WorkspaceDir, kind Kind, req Request) (Job, error) {
	return Job{}, xerrors.User("not_supported", "background jobs are not supported on Windows in v0.3", "use foreground review")
}

func Cancel(ws state.WorkspaceDir, id string) (Job, error) {
	return Job{}, xerrors.User("not_supported", "background jobs are not supported on Windows in v0.3", "")
}

func SweepOrphans(ws state.WorkspaceDir) {}
