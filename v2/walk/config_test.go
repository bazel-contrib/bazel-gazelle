package walk

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
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

func TestConfigurerDirectives(t *testing.T) {
	c := config.New()
	cc := &Configurer{}
	buildData := []byte(`# gazelle:build_file_name x,y`)
	f, err := rule.LoadData(filepath.Join("test", "BUILD.bazel"), "", buildData)
	if err != nil {
		t.Fatal(err)
	}
	if err := cc.CheckFlags(nil, c); err != nil {
		t.Errorf("CheckFlags: %v", err)
	}
	cc.Configure(t.Context(), config.ConfigureArgs{Config: c, Rel: "", File: f})
	want := []string{"x", "y"}
	if !reflect.DeepEqual(c.ValidBuildFileNames, want) {
		t.Errorf("for ValidBuildFileNames, got %#v, want %#v", c.ValidBuildFileNames, want)
	}
}

func TestAddExcludes(t *testing.T) {
	wc := &walkConfig{}
	wc.addExcludes([]string{
		"a/b.txt", // plain literal
		"caffè",   // non-ASCII literal
		"*.go",    // star
		"a?b",     // question mark
		"[abc]",   // character class
		"{l,r}",   // alternatives
		`e\.go`,   // escape
	})

	wantLiterals := map[string]struct{}{"a/b.txt": {}, "caffè": {}}
	if !reflect.DeepEqual(wc.literalExcludes, wantLiterals) {
		t.Errorf("for literalExcludes, got %v, want %v", wc.literalExcludes, wantLiterals)
	}
	wantGlobs := []string{"*.go", "a?b", "[abc]", "{l,r}", `e\.go`}
	if !reflect.DeepEqual(wc.globExcludes, wantGlobs) {
		t.Errorf("for globExcludes, got %v, want %v", wc.globExcludes, wantGlobs)
	}

	// Adding literals to a clone must not modify the shared set.
	wc.clone().addExcludes([]string{"c.txt"})
	if !reflect.DeepEqual(wc.literalExcludes, wantLiterals) {
		t.Errorf("after adding to a clone, got literalExcludes %v, want %v", wc.literalExcludes, wantLiterals)
	}
}

func TestIsExcludedByPattern(t *testing.T) {
	// Literals are matched with a map lookup, glob and escaped patterns with
	// the doublestar matcher.
	mixed := &walkConfig{}
	mixed.addExcludes([]string{"a/b.txt", "x.gen", "*.pb.go", `e\.go`, "caffè"})
	globOnly := &walkConfig{}
	globOnly.addExcludes([]string{"*.pb.go"})

	for _, tc := range []struct {
		wc   *walkConfig
		path string
		want bool
	}{
		{mixed, "a/b.txt", true}, // literal
		{mixed, "x.gen", true},   // literal
		{mixed, "y.pb.go", true}, // glob
		{mixed, "e.go", true},    // escaped literal
		{mixed, "caffè", true},   // non-ASCII literal
		{mixed, "a/b", false},
		{mixed, "y.gen", false},
		{globOnly, "y.pb.go", true},
		{globOnly, "y.go", false},
	} {
		if got := tc.wc.isExcludedByPattern(tc.path); got != tc.want {
			t.Errorf("isExcludedByPattern(%q) = %t, want %t", tc.path, got, tc.want)
		}
	}
}

// BenchmarkExclusions measures the cost of checking a path that matches no
// exclusion, with varying numbers of literal (non-glob) exclude patterns
// configured through Configurer.CheckFlags, as -exclude flags would.
func BenchmarkExclusions(b *testing.B) {
	for _, count := range []int{0, 1, 100, 1000} {
		b.Run(fmt.Sprintf("literals=%d", count), func(b *testing.B) {
			args := make([]string, 0, 2*count)
			for i := range count {
				args = append(args, "-exclude", fmt.Sprintf("excluded%d", i))
			}

			c := config.New()
			c.RepoRoot = b.TempDir()
			cc := &Configurer{}
			fs := flag.NewFlagSet("bench", flag.ContinueOnError)
			cc.RegisterFlags(fs, "update", c)
			if err := fs.Parse(args); err != nil {
				b.Fatal(err)
			}
			if err := cc.CheckFlags(fs, c); err != nil {
				b.Fatal(err)
			}
			wc := getWalkConfig(c)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if wc.isExcludedDir("some/unexcluded/dir") {
					b.Fatal("path unexpectedly excluded")
				}
			}
		})
	}
}
