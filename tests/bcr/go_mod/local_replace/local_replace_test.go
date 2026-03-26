package local_replace_test

import (
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/internal/wspace"
)

// TestLocalReplace verifies that go_deps honors local-path replace directives
// from non-root Bazel modules. The v2 sub-module is declared in gazelle's own
// go.mod with `replace => ./v2`, which is a non-root replace from the
// perspective of any downstream workspace. Without the fix, go_repository is
// called without local_path and cannot fetch the module.
func TestLocalReplace(t *testing.T) {
	// Calling any function from the package is enough to prove the repo
	// resolved correctly.
	if _, err := wspace.FindRepoRoot(t.TempDir()); err == nil {
		t.Fatal("expected FindRepoRoot to fail on an empty dir, got nil")
	}
}
