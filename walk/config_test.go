package walk

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bmatcuk/doublestar/v4"
	bzl "github.com/bazelbuild/buildtools/build"
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
	cc.Configure(c, "", f)
	want := []string{"x", "y"}
	if !reflect.DeepEqual(c.ValidBuildFileNames, want) {
		t.Errorf("for ValidBuildFileNames, got %#v, want %#v", c.ValidBuildFileNames, want)
	}
}

func TestCollectStringsFromExpr(t *testing.T) {
	testCases := []struct {
		name    string
		expr    bzl.Expr
		vars    map[string]bzl.Expr
		want    []string
		wantErr bool
	}{
		{
			name: "plain list",
			expr: &bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: "dir1"},
					&bzl.StringExpr{Value: "dir2"},
				},
			},
			vars:    nil,
			want:    []string{"dir1", "dir2"},
			wantErr: false,
		},
		{
			name: "variable reference",
			expr: &bzl.Ident{Name: "_DIRS"},
			vars: map[string]bzl.Expr{
				"_DIRS": &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "vendor"},
						&bzl.StringExpr{Value: "third_party"},
					},
				},
			},
			want:    []string{"vendor", "third_party"},
			wantErr: false,
		},
		{
			name: "binary concatenation of two lists",
			expr: &bzl.BinaryExpr{
				Op: "+",
				X: &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir1"},
					},
				},
				Y: &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir2"},
					},
				},
			},
			vars:    nil,
			want:    []string{"dir1", "dir2"},
			wantErr: false,
		},
		{
			name: "variable concatenation pattern",
			expr: &bzl.BinaryExpr{
				Op:   "+",
				X:    &bzl.Ident{Name: "_FOO"},
				Y:    &bzl.Ident{Name: "_BAR"},
			},
			vars: map[string]bzl.Expr{
				"_FOO": &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir1"},
						&bzl.StringExpr{Value: "dir2"},
					},
				},
				"_BAR": &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir3"},
					},
				},
			},
			want:    []string{"dir1", "dir2", "dir3"},
			wantErr: false,
		},
		{
			name:    "unresolved variable",
			expr:    &bzl.Ident{Name: "_UNKNOWN"},
			vars:    map[string]bzl.Expr{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "unsupported binary operator",
			expr: &bzl.BinaryExpr{
				Op: "-",
				X: &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir1"},
					},
				},
				Y: &bzl.ListExpr{
					List: []bzl.Expr{
						&bzl.StringExpr{Value: "dir2"},
					},
				},
			},
			vars:    nil,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectStringsFromExpr(tc.expr, tc.vars)
			if (err != nil) != tc.wantErr {
				t.Errorf("collectStringsFromExpr() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("collectStringsFromExpr() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadRepoDirectoryIgnoreWithVariables(t *testing.T) {
	dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "repo_ignore_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	repoContent := []byte(`
_GENERATED = ["generated1", "generated2"]
_VENDOR = ["vendor"]

ignore_directories(_GENERATED + _VENDOR)
`)
	if err := os.WriteFile(filepath.Join(dir, "REPO.bazel"), repoContent, 0o666); err != nil {
		t.Fatal(err)
	}

	got, err := loadRepoDirectoryIgnore(dir)
	if err != nil {
		t.Fatalf("loadRepoDirectoryIgnore() error: %v", err)
	}

	want := []string{"generated1", "generated2", "vendor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loadRepoDirectoryIgnore() = %v, want %v", got, want)
	}
}

func TestBuildVarMap(t *testing.T) {
	stmts := []bzl.Expr{
		&bzl.AssignExpr{
			LHS: &bzl.Ident{Name: "_FOO"},
			Op:  "=",
			RHS: &bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: "a"},
				},
			},
		},
		&bzl.AssignExpr{
			LHS: &bzl.Ident{Name: "_BAR"},
			Op:  "=",
			RHS: &bzl.ListExpr{
				List: []bzl.Expr{
					&bzl.StringExpr{Value: "b"},
				},
			},
		},
	}

	vars := buildVarMap(stmts)
	if len(vars) != 2 {
		t.Fatalf("buildVarMap() returned %d entries, want 2", len(vars))
	}
	if _, ok := vars["_FOO"]; !ok {
		t.Error("buildVarMap() missing _FOO")
	}
	if _, ok := vars["_BAR"]; !ok {
		t.Error("buildVarMap() missing _BAR")
	}
}
