//go:build !lite

package treesitter

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// NodeDef is a simple (name, symbolKind, startByte, endByte) tuple returned
// by WalkNamedDefs. symbolKind is the node's type ID as returned by the grammar.
type NodeDef struct {
	NameStart uint32
	NameEnd   uint32
}

// FieldIDForName looks up a field name's numeric ID in the language's field
// table. Returns 0 if not found (field IDs start at 1).
func (l *Language) FieldIDForName(ctx context.Context, fieldName string) uint32 {
	r := l.rt
	fieldCountFn := r.tsMod.ExportedFunction("ts_language_field_count")
	fieldNameFn := r.tsMod.ExportedFunction("ts_language_field_name_for_id")
	if fieldCountFn == nil || fieldNameFn == nil {
		return 0
	}

	res, err := fieldCountFn.Call(ctx, uint64(l.langPtr))
	if err != nil {
		return 0
	}
	count := api.DecodeU32(res[0])

	for i := uint32(1); i <= count; i++ {
		nameRes, err := fieldNameFn.Call(ctx, uint64(l.langPtr), uint64(i))
		if err != nil {
			continue
		}
		namePtr := api.DecodeU32(nameRes[0])
		if namePtr == 0 {
			continue
		}
		var bs []byte
		for off := namePtr; ; off++ {
			b, ok := r.mem.ReadByte(off)
			if !ok || b == 0 {
				break
			}
			bs = append(bs, b)
		}
		if string(bs) == fieldName {
			return i
		}
	}
	return 0
}

// SymbolIDForName looks up a node type's numeric ID in the language's symbol
// table. Returns 0 if not found. Pass named=true to search named symbols only.
func (l *Language) SymbolIDForName(ctx context.Context, name string, named bool) uint16 {
	r := l.rt
	fn := r.tsMod.ExportedFunction("ts_language_symbol_for_name")
	if fn == nil {
		return 0
	}

	bs := append([]byte(name), 0)
	mallocRes, err := r.rfns.malloc.Call(ctx, uint64(len(bs)))
	if err != nil {
		return 0
	}
	ptr := api.DecodeU32(mallocRes[0])
	defer r.rfns.free.Call(ctx, uint64(ptr)) //nolint:errcheck
	r.mem.Write(ptr, bs)

	namedInt := uint64(0)
	if named {
		namedInt = 1
	}
	res, err := fn.Call(ctx, uint64(l.langPtr), uint64(ptr), uint64(len(name)), namedInt)
	if err != nil {
		return 0
	}
	return uint16(api.DecodeU32(res[0]))
}

// sizeOfCursor is the size of a TSTreeCursor struct in wasm memory: 3 × i32 = 12 bytes.
const sizeOfCursor = 12

// WalkNamedChildren walks all named nodes matching one of the given symbol IDs
// using a depth-first tree cursor and calls fn for each matching node to get
// its name-field child's byte range. Only nodes with a non-empty name child
// are reported.
//
// If nameFieldID > 0, for each matching node, the "name" field child is looked
// up and its byte range is returned. If nameFieldID == 0, the node's own byte
// range is used.
//
// Returns a slice of NodeDef for each matching (name start/end byte) found.
// Avoids ts_query_new entirely — safe to call in any runtime state.
func (l *Language) WalkNamedChildren(ctx context.Context, tree *Tree, matchSymIDs []uint16, nameFieldID uint32) ([]NodeDef, error) {
	r := l.rt
	mem := r.mem

	cursorNewFn := r.tsMod.ExportedFunction("ts_tree_cursor_new_wasm")
	cursorDeleteFn := r.tsMod.ExportedFunction("ts_tree_cursor_delete_wasm")
	cursorFirstChildFn := r.tsMod.ExportedFunction("ts_tree_cursor_goto_first_child_wasm")
	cursorNextSiblingFn := r.tsMod.ExportedFunction("ts_tree_cursor_goto_next_sibling_wasm")
	cursorParentFn := r.tsMod.ExportedFunction("ts_tree_cursor_goto_parent_wasm")
	cursorCurrentNodeFn := r.tsMod.ExportedFunction("ts_tree_cursor_current_node_wasm")
	cursorTypeIDFn := r.tsMod.ExportedFunction("ts_tree_cursor_current_node_type_id_wasm")
	childByFieldFn := r.tsMod.ExportedFunction("ts_node_child_by_field_id_wasm")
	endIndexFn := r.tsMod.ExportedFunction("ts_node_end_index_wasm")

	if cursorNewFn == nil || cursorDeleteFn == nil || cursorFirstChildFn == nil ||
		cursorNextSiblingFn == nil || cursorParentFn == nil || cursorCurrentNodeFn == nil ||
		cursorTypeIDFn == nil || endIndexFn == nil {
		return nil, fmt.Errorf("treesitter: required cursor functions not exported")
	}

	// Place root node in TRANSFER_BUFFER and create cursor.
	root := tree.RootNode(ctx)
	root.marshalNodeToTransferBuf()
	if _, err := cursorNewFn.Call(ctx, uint64(tree.treePtr)); err != nil {
		return nil, fmt.Errorf("treesitter: ts_tree_cursor_new_wasm: %w", err)
	}

	// Save cursor state (12 bytes at TRANSFER_BUFFER).
	cursor := make([]byte, sizeOfCursor)
	cursorRaw, ok := mem.Read(r.transferBuf, sizeOfCursor)
	if !ok {
		return nil, fmt.Errorf("treesitter: read cursor from TRANSFER_BUFFER failed")
	}
	copy(cursor, cursorRaw)

	// Delete cursor on return.
	defer func() {
		mem.Write(r.transferBuf, cursor)
		cursorDeleteFn.Call(ctx) //nolint:errcheck
	}()

	// Build a set of matching symbol IDs for fast lookup.
	matchSet := make(map[uint16]bool, len(matchSymIDs))
	for _, id := range matchSymIDs {
		matchSet[id] = true
	}

	var results []NodeDef
	depth := 0

	for {
		// Restore cursor into TRANSFER_BUFFER.
		mem.Write(r.transferBuf, cursor)

		// Get current node type ID.
		typeRes, err := cursorTypeIDFn.Call(ctx, uint64(tree.treePtr))
		if err != nil {
			break
		}
		typeID := uint16(api.DecodeU32(typeRes[0]))

		if matchSet[typeID] {
			// Get current node raw bytes.
			mem.Write(r.transferBuf, cursor)
			if _, err := cursorCurrentNodeFn.Call(ctx, uint64(tree.treePtr)); err == nil {
				nodeRaw, ok := mem.Read(r.transferBuf, sizeOfNode)
				if ok {
					if nameFieldID > 0 && childByFieldFn != nil {
						// Navigate to the "name" field child.
						mem.Write(r.transferBuf, nodeRaw)
						if _, err := childByFieldFn.Call(ctx, uint64(tree.treePtr), uint64(nameFieldID)); err == nil {
							nameNodeRaw, ok := mem.Read(r.transferBuf, sizeOfNode)
							if ok {
								nameStart := binary.LittleEndian.Uint32(nameNodeRaw[4:8])
								mem.Write(r.transferBuf, nameNodeRaw)
								endRes, err := endIndexFn.Call(ctx, uint64(tree.treePtr))
								if err == nil {
									nameEnd := api.DecodeU32(endRes[0])
									if nameStart < nameEnd {
										results = append(results, NodeDef{NameStart: nameStart, NameEnd: nameEnd})
									}
								}
							}
						}
					} else {
						// Use node's own byte range.
						nodeStart := binary.LittleEndian.Uint32(nodeRaw[4:8])
						mem.Write(r.transferBuf, nodeRaw)
						endRes, err := endIndexFn.Call(ctx, uint64(tree.treePtr))
						if err == nil {
							nodeEnd := api.DecodeU32(endRes[0])
							if nodeStart < nodeEnd {
								results = append(results, NodeDef{NameStart: nodeStart, NameEnd: nodeEnd})
							}
						}
					}
				}
			}
		}

		// Restore cursor for navigation.
		mem.Write(r.transferBuf, cursor)

		// Try to go to first child.
		childRes, err := cursorFirstChildFn.Call(ctx, uint64(tree.treePtr))
		if err == nil && api.DecodeI32(childRes[0]) != 0 {
			cursorRaw, ok = mem.Read(r.transferBuf, sizeOfCursor)
			if !ok {
				break
			}
			copy(cursor, cursorRaw)
			depth++
			continue
		}

		// No children — try next sibling.
		for {
			mem.Write(r.transferBuf, cursor)
			sibRes, err := cursorNextSiblingFn.Call(ctx, uint64(tree.treePtr))
			if err == nil && api.DecodeI32(sibRes[0]) != 0 {
				cursorRaw, ok = mem.Read(r.transferBuf, sizeOfCursor)
				if !ok {
					goto done
				}
				copy(cursor, cursorRaw)
				break
			}
			// No next sibling — go to parent.
			if depth == 0 {
				goto done
			}
			mem.Write(r.transferBuf, cursor)
			parentRes, err := cursorParentFn.Call(ctx, uint64(tree.treePtr))
			if err != nil || api.DecodeI32(parentRes[0]) == 0 {
				goto done
			}
			cursorRaw, ok = mem.Read(r.transferBuf, sizeOfCursor)
			if !ok {
				goto done
			}
			copy(cursor, cursorRaw)
			depth--
		}
	}
done:
	return results, nil
}
