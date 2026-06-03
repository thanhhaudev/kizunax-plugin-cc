;; env module — provides memory + table + globals + libc-style functions
;; for both the web-tree-sitter runtime AND the PHP grammar side module.
;; All function imports come from "host"; we re-export them under "env".

(module
  ;; Runtime-needed callbacks
  (import "host" "emscripten_resize_heap" (func $resize (param i32) (result i32)))
  (import "host" "_abort_js" (func $abort))
  (import "host" "tree_sitter_query_progress_callback" (func $qprog (param i32) (result i32)))
  (import "host" "tree_sitter_progress_callback" (func $prog (param i32 i32) (result i32)))
  (import "host" "tree_sitter_parse_callback" (func $parse (param i32 i32 i32 i32 i32)))
  (import "host" "tree_sitter_log_callback" (func $log (param i32 i32)))

  ;; Grammar-needed libc functions (forwarded by host trampolines to runtime exports)
  (import "host" "calloc"       (func $calloc       (param i32 i32) (result i32)))
  (import "host" "malloc"       (func $malloc       (param i32) (result i32)))
  (import "host" "free"         (func $free         (param i32)))
  (import "host" "realloc"      (func $realloc      (param i32 i32) (result i32)))
  (import "host" "memcpy"       (func $memcpy       (param i32 i32 i32) (result i32)))
  (import "host" "memcmp"       (func $memcmp       (param i32 i32 i32) (result i32)))
  (import "host" "iswspace"     (func $iswspace     (param i32) (result i32)))
  (import "host" "iswxdigit"    (func $iswxdigit    (param i32) (result i32)))
  (import "host" "iswalnum"     (func $iswalnum     (param i32) (result i32)))
  (import "host" "__assert_fail" (func $assfail     (param i32 i32 i32 i32)))

  (memory (export "memory") 512 32768)

  (global (export "__stack_pointer") (mut i32) (i32.const 65536))
  (global (export "__memory_base") i32 (i32.const 0))
  (global (export "__table_base") i32 (i32.const 0))

  (table (export "__indirect_function_table") 1024 funcref)

  (export "emscripten_resize_heap" (func $resize))
  (export "_abort_js" (func $abort))
  (export "tree_sitter_query_progress_callback" (func $qprog))
  (export "tree_sitter_progress_callback" (func $prog))
  (export "tree_sitter_parse_callback" (func $parse))
  (export "tree_sitter_log_callback" (func $log))

  (export "calloc" (func $calloc))
  (export "malloc" (func $malloc))
  (export "free" (func $free))
  (export "realloc" (func $realloc))
  (export "memcpy" (func $memcpy))
  (export "memcmp" (func $memcmp))
  (export "iswspace" (func $iswspace))
  (export "iswxdigit" (func $iswxdigit))
  (export "iswalnum" (func $iswalnum))
  (export "__assert_fail" (func $assfail))
)
