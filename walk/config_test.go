package walk

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bmatcuk/doublestar/v4"
)

func TestCheckPathMatchPattern(t *testing.T) {
	testCases := []struct {
		pattern string
		err     error
	}{
		{pattern: "*.pb.go", err: nil},
		{pattern: "**/*.pb.go", err: nil},
		{pattern: "**/*.pb.go", err: nil},
		{pattern: "[]a]", err: doublestar.ErrBadPattern},
		{pattern: "[c-", err: doublestar.ErrBadPattern},
	}

	for _, testCase := range testCases {
		if want, got := testCase.err, checkPathMatchPattern(testCase.pattern); want != got {
			t.Errorf("checkPathMatchPattern %q: got %q want %q", testCase.pattern, got, want)
		}
	}
}

func TestConfigurerFlags(t *testing.T) {
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "WORKSPACE"), nil, 0o666); err != nil {
		t.Fatal(err)
	}

	c := config.New()
	cc := &Configurer{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cc.RegisterFlags(fs, "test", c)
	args := []string{"-build_file_name", "x,y"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if err := cc.CheckFlags(fs, c); err != nil {
		t.Errorf("CheckFlags: %v", err)
	}

	wantBuildFileNames := []string{"x", "y"}
	if !reflect.DeepEqual(c.ValidBuildFileNames, wantBuildFileNames) {
		t.Errorf("for ValidBuildFileNames, got %#v, want %#v", c.ValidBuildFileNames, wantBuildFileNames)
	}
}
