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
// Tasks 3–6 complete. Task 7 adds grammar loading (Language type).
// Task 8 will add the query API (Query type).
type Runtime struct {
	wazRt       wazero.Runtime
	rfns        *runtimeFns
	tsMod       api.Module
	envMod      api.Module // current "env" module (may be replaced during grammar loads)
	mem         api.Memory // shared memory from mem_owner; stable across env module swaps
	transferBuf uint32     // address returned by ts_init(); shared TRANSFER_BUFFER for all _wasm calls
}

// parseSrc holds the source bytes for the current in-progress parse.
// Protected by parseSrcMu; safe because the runtime is a process-wide
// singleton and we serialize parse calls.
var (
	parseSrcMu  sync.Mutex
	parseSrcBuf []byte
)

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

	// mem_owner.wasm must be instantiated FIRST. It owns the shared memory
	// and function table that all other modules (env, grammar env, runtime,
	// grammars) import. This ensures a single shared memory instance.
	if _, err := rt.InstantiateWithConfig(ctx, MemOwnerWASM,
		wazero.NewModuleConfig().WithName("mem_owner")); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: mem_owner instantiate: %w", err)
	}

	// env.wasm — imports memory/table from mem_owner, provides __memory_base=0
	// and function re-exports for the runtime module.
	envMod, err := rt.InstantiateWithConfig(ctx, EnvWASM,
		wazero.NewModuleConfig().WithName("env"))
	if err != nil {
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

	// __wasm_apply_data_relocs must run before __wasm_call_ctors and ts_init.
	// It patches function-pointer entries in the data segment that Emscripten
	// dynamic-linking stores as integer offsets from __table_base rather than
	// direct funcref table elements.
	applyRelocs := tsMod.ExportedFunction("__wasm_apply_data_relocs")
	if applyRelocs != nil {
		if _, err := applyRelocs.Call(ctx); err != nil {
			_ = rt.Close(ctx)
			return nil, fmt.Errorf("treesitter: __wasm_apply_data_relocs: %w", err)
		}
	}

	// __wasm_call_ctors runs C++ constructors and any runtime-level
	// initialization that couldn't happen in the start section.
	ctors := tsMod.ExportedFunction("__wasm_call_ctors")
	if ctors != nil {
		if _, err := ctors.Call(ctx); err != nil {
			_ = rt.Close(ctx)
			return nil, fmt.Errorf("treesitter: __wasm_call_ctors: %w", err)
		}
	}

	tsInit := tsMod.ExportedFunction("ts_init")
	if tsInit == nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: ts_init not exported")
	}
	initRes, err := tsInit.Call(ctx)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: ts_init call: %w", err)
	}
	// ts_init() returns the TRANSFER_BUFFER address — the shared memory
	// region used by all _wasm suffix functions to pass results between
	// Go and the wasm runtime.
	transferBuf := api.DecodeU32(initRes[0])
	if transferBuf == 0 {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: ts_init returned NULL transfer buffer")
	}

	// Obtain the shared memory from mem_owner. This reference is stable even
	// if the "env" module is closed and replaced during grammar loading.
	memOwnerMod := rt.Module("mem_owner")
	if memOwnerMod == nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: mem_owner module not found after instantiation")
	}
	sharedMem := memOwnerMod.ExportedMemory("memory")
	if sharedMem == nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("treesitter: mem_owner does not export memory")
	}

	return &Runtime{wazRt: rt, rfns: rfns, tsMod: tsMod, envMod: envMod, mem: sharedMem, transferBuf: transferBuf}, nil
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
		WithFunc(func(ctx context.Context, mod api.Module, inputBuf, byteIndex, row, column, lenAddr int32) {
			// Serve the bytes at byteIndex from the current parse source into
			// inputBuf, writing the byte-count at lenAddr. This is the mechanism
			// by which the wasm parser reads source text incrementally.
			parseSrcMu.Lock()
			src := parseSrcBuf
			parseSrcMu.Unlock()
			if byteIndex < 0 || int(byteIndex) >= len(src) {
				// Signal EOF or out-of-range.
				mod.Memory().WriteUint32Le(uint32(lenAddr), 0)
				return
			}
			chunk := src[byteIndex:]
			const maxChunk = 10 * 1024
			if len(chunk) > maxChunk {
				chunk = chunk[:maxChunk]
			}
			mod.Memory().Write(uint32(inputBuf), chunk)
			mod.Memory().WriteUint32Le(uint32(lenAddr), uint32(len(chunk)))
		}).
		Export("tree_sitter_parse_callback").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, logType, message int32) {}).
		Export("tree_sitter_log_callback")

	b = addTrampolines(b, rfns)
	_, err := b.Instantiate(ctx)
	return err
}
