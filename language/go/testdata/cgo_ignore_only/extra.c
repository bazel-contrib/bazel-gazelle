/* Stray C file next to pure Go: if go_library were generated with cgo=True by
   mistake, Gazelle would add this to srcs. The golden BUILD.want asserts it does not. */

void orphan(void) {}
