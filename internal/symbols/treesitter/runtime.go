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
	api "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime holds the wazero runtime + the instantiated tree-sitter
// module. Use getRuntime to obtain the process-wide singleton.
//
// Tasks 3–6 complete. Subsequent tasks fill in:
//   - Task 7: grammar loading (Language type)
//   - Task 8: query API (Query type)
type Runtime struct {
	wazRt wazero.Runtime
	rfns  *runtimeFns
	tsMod api.Module
}

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

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: wasi instantiate: %w", err)
	}

	rfns := &runtimeFns{}
	if err := instantiateHostModule(ctx, rt, rfns); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: host instantiate: %w", err)
	}

	// env.wasm + GOT.mem.wasm — both must be instantiated BEFORE the
	// runtime so its imports resolve.
	if _, err := rt.InstantiateWithConfig(ctx, EnvWASM,
		wazero.NewModuleConfig().WithName("env")); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: env instantiate: %w", err)
	}
	if _, err := rt.InstantiateWithConfig(ctx, GOTMemWASM,
		wazero.NewModuleConfig().WithName("GOT.mem")); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: GOT.mem instantiate: %w", err)
	}

	tsMod, err := rt.InstantiateWithConfig(ctx, runtimeWASM,
		wazero.NewModuleConfig().WithName("tree-sitter"))
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: runtime instantiate: %w", err)
	}

	if err := bindRuntimeFns(rfns, tsMod); err != nil {
		_ = rt.Close(ctx)
		return nil, err
	}

	tsInit := tsMod.ExportedFunction("ts_init")
	if tsInit == nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: ts_init not exported")
	}
	if _, err := tsInit.Call(ctx); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: ts_init call: %w", err)
	}

	return &Runtime{wazRt: rt, rfns: rfns, tsMod: tsMod}, nil
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
