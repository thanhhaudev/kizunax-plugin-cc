//go:build !lite

// Package treesitter wraps the web-tree-sitter runtime via wazero to
// extract structured symbols from non-Go source files. The runtime is
// a process-wide singleton instantiated lazily.
package treesitter

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime holds the wazero runtime + the instantiated tree-sitter
// module. Use getRuntime to obtain the process-wide singleton.
//
// Task 3 scope: skeleton only. Subsequent tasks fill in:
//   - Task 4: env.wasm + got_mem.wasm instantiation
//   - Task 5: host trampolines + runtimeFns late-binding
//   - Task 6: web-tree-sitter.wasm runtime instantiation + ts_init
//   - Task 7: grammar loading (Language type)
//   - Task 8: query API (Query type)
type Runtime struct {
	wazRt wazero.Runtime
	rfns  *runtimeFns
	tsMod wazeroModule // placeholder until Task 6 fills in api.Module
}

// wazeroModule is a placeholder until Task 6 replaces it with api.Module.
// This lets the package compile while the runtime is built up incrementally.
type wazeroModule interface{}

var (
	runtimeOnce sync.Once
	runtimeInst *Runtime
	runtimeErr  error
)

// getRuntime returns the process-wide tree-sitter runtime singleton.
// First call lazily constructs the runtime; subsequent calls return
// the cached instance (or the cached error if first call failed).
func getRuntime(ctx context.Context) (*Runtime, error) {
	runtimeOnce.Do(func() {
		runtimeInst, runtimeErr = newRuntime(ctx)
	})
	return runtimeInst, runtimeErr
}

func newRuntime(ctx context.Context) (*Runtime, error) {
	rt := wazero.NewRuntime(ctx)

	// WASI snapshot preview 1 — provides fd_write / fd_seek / fd_close
	// that the runtime needs (rarely called in practice).
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: wasi instantiate: %w", err)
	}

	rfns := &runtimeFns{}
	if err := instantiateHostModule(ctx, rt, rfns); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: host instantiate: %w", err)
	}

	return &Runtime{wazRt: rt, rfns: rfns}, nil
}

// instantiateHostModule wires up the "host" module that env.wasm imports
// from. Provides:
//   - 6 tree-sitter / emscripten callback stubs (no-ops; runtime won't
//     normally invoke them).
//   - 10 libc trampolines (via addTrampolines) that forward to runtime
//     exports via the runtimeFns late-bound struct.
//
// The trampolines fail-loud (panic) if invoked before bindRuntimeFns
// has populated rfns — but this never happens in practice because no
// grammar can run before the runtime module is up.
func instantiateHostModule(ctx context.Context, rt wazero.Runtime, rfns *runtimeFns) error {
	b := rt.NewHostModuleBuilder("host").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, requestedPages int32) int32 { return 0 }).
		Export("emscripten_resize_heap").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context) {
			panic("treesitter: _abort_js invoked by runtime")
		}).
		Export("_abort_js").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, payload int32) int32 { return 0 }).
		Export("tree_sitter_query_progress_callback").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, payload, isError int32) int32 { return 0 }).
		Export("tree_sitter_progress_callback").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, payload, byteIndex, row, column, bufLen int32) {}).
		Export("tree_sitter_parse_callback").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, logType, message int32) {}).
		Export("tree_sitter_log_callback")

	b = addTrampolines(b, rfns)
	_, err := b.Instantiate(ctx)
	return err
}
