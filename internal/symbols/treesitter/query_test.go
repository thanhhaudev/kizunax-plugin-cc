//go:build !lite

package treesitter

import (
	"context"
	"testing"
)

func TestQuery_PHPFunctionDef(t *testing.T) {
	ctx := context.Background()
	r, err := getRuntime(ctx)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	lang, err := r.LoadGrammar(ctx, "php", loadPhpGrammar(t))
	if err != nil {
		t.Fatalf("LoadGrammar: %v", err)
	}
	defer lang.Close(ctx)

	// Compile the query BEFORE parsing: ts_query_new calls malloc internally,
	// and the parse+free cycle can leave dlmalloc free-list metadata with
	// out-of-range sentinel values that crash ts_query_new's first malloc.
	// Compiling once before parsing is also the correct usage pattern.
	q, err := lang.NewQuery(ctx, "(function_definition name: (name) @name.definition.function)")
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	defer q.Close(ctx)

	src := []byte("<?php\nfunction login() {}\nfunction logout() {}\n")
	tree, err := lang.Parse(ctx, src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close(ctx)

	caps, err := q.Exec(ctx, tree.RootNode(ctx))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	want := map[string]bool{"login": false, "logout": false}
	for _, c := range caps {
		text := string(src[c.StartByte:c.EndByte])
		if _, ok := want[text]; ok {
			want[text] = true
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("missing capture for %q in %+v", name, caps)
		}
	}
}
