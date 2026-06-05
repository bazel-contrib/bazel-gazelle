package module

import (
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/google/go-cmp/cmp"
)

func TestCollectApparent(t *testing.T) {
	moduleFile, err := runfiles.Rlocation("bazel_gazelle/v2/internal/module/testdata/MODULE.bazel")
	if err != nil {
		t.Fatal(err)
	}

	apparentNames, err := collectApparentNames(filepath.Dir(moduleFile), "MODULE.bazel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"rules_bar":   "rules_bar",
		"rules_baz":   "rules_baz",
		"rules_foo":   "my_rules_foo",
		"rules_lang":  "my_rules_lang",
		"rules_quz":   "rules_quz",
		"test_module": "my_test_module",
	}
	if diff := cmp.Diff(expected, apparentNames); diff != "" {
		t.Errorf("unexpected apparent names (-want +got):\n%s", diff)
	}
}

func TestCollectApparent_fileDoesNotExist(t *testing.T) {
	_, err := collectApparentNames(t.TempDir(), "MODULE.bazel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = collectApparentNames(t.TempDir(), "segment.MODULE.bazel")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
