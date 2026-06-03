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
	tsMod wazeroModule // placeholder type until Task 6 fills in api.Module
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

	// Host module with the 6 runtime callback stubs.
	if err := instantiateHostModule(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: host instantiate: %w", err)
	}

	return &Runtime{wazRt: rt}, nil
}

// instantiateHostModule wires up the "host" module that env.wasm imports
// from. The 6 runtime callbacks are stubs that do nothing useful but
// must exist to satisfy the runtime's imports.
//
// The 10 libc stubs (calloc, malloc, free, realloc, memcpy, memcmp,
// iswspace, iswxdigit, iswalnum, __assert_fail) are also present here
// as zero-value placeholders. Task 5 replaces them with proper trampolines
// that forward to runtime exports via the runtimeFns late-bound struct.
func instantiateHostModule(ctx context.Context, rt wazero.Runtime) error {
	_, err := rt.NewHostModuleBuilder("host").
		// Tree-sitter runtime callbacks.
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, requestedPages int32) int32 { return 0 }).
		Export("emscripten_resize_heap").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context) {
			// Runtime should never call this in normal operation;
			// it indicates a fatal C-level abort.
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
		Export("tree_sitter_log_callback").
		// libc stubs — zero-value placeholders. Task 5 replaces these with
		// trampolines that delegate to the runtime module's own exports.
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, nmemb, size int32) int32 { return 0 }).
		Export("calloc").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, size int32) int32 { return 0 }).
		Export("malloc").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, ptr int32) {}).
		Export("free").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, ptr, size int32) int32 { return 0 }).
		Export("realloc").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, dst, src, n int32) int32 { return dst }).
		Export("memcpy").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, s1, s2, n int32) int32 { return 0 }).
		Export("memcmp").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, wc int32) int32 { return 0 }).
		Export("iswspace").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, wc int32) int32 { return 0 }).
		Export("iswxdigit").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, wc int32) int32 { return 0 }).
		Export("iswalnum").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, assertion, file, line, fn int32) {
			panic("treesitter: __assert_fail invoked by runtime")
		}).
		Export("__assert_fail").
		Instantiate(ctx)
	return err
}
