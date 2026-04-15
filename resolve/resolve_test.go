package resolve

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/google/go-cmp/cmp"
)

func TestFindRuleWithOverride_ParentTraversal(t *testing.T) {
	rootCfg := getConfig(t, "", []rule.Directive{
		{Key: "resolve", Value: "go go github.com/root/repo @com_example//root:replacement"},
		{Key: "resolve_regexp", Value: "go ^github.com/root/(.*)$ @com_example//regexp:replacement"},
	}, nil)

	childCfg := getConfig(t, "child/rel", []rule.Directive{
		{Key: "resolve", Value: "go github.com/child/repo //some/local/child:replacement"},
		{Key: "resolve_regexp", Value: "go ^github.com/child/(.*)$ relative/child/regexp"},
	}, rootCfg)

	secondChildCfg := getConfig(t, "second/child/rel", nil, rootCfg)

	dualResolveRegexpCfg := getConfig(t, "dual/resolve/regexp", []rule.Directive{
		{Key: "resolve_regexp", Value: "go ^github.com/foo/(.*)$ @com_example//$1:replacement"},
	}, rootCfg)

	multipleExpDualResolveRegexpCfg := getConfig(t, "multi/dual/resolve/regexp", []rule.Directive{
		{Key: "resolve_regexp", Value: "go ^github.com/foo/(.*)/(.*)$ @com_example//$1/bar_sub_dir/$2:replacement"},
	}, rootCfg)

	nonCapturedRegexpCfg := getConfig(t, "", []rule.Directive{
		{Key: "resolve_regexp", Value: "py github.com/\\.* @com_example//regexp:no_replacement"},
	}, nil)

	tests := []struct {
		name      string
		cfg       *config.Config
		importSpec ImportSpec
		lang      string
		want      label.Label
		wantFound bool
	}{
		{
			name:      "Child exact match",
			cfg:       childCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/child/repo"},
			lang:      "go",
			want:      getTestLabel(t, "//some/local/child:replacement"),
			wantFound: true,
		},
		{
			name:      "Child regexp match",
			cfg:       childCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/child/other"},
			lang:      "go",
			want:      getTestLabel(t, "//child/rel:relative/child/regexp"),
			wantFound: true,
		},
		{
			name:      "Root exact match from child",
			cfg:       childCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/root/repo"},
			lang:      "go",
			want:      getTestLabel(t, "@com_example//root:replacement"),
			wantFound: true,
		},
		{
			name:      "Root regexp match from child",
			cfg:       childCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/root/some"},
			lang:      "go",
			want:      getTestLabel(t, "@com_example//regexp:replacement"),
			wantFound: true,
		},
		{
			name:      "No match in child or root",
			cfg:       childCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/nonexistent/repo"},
			lang:      "go",
			want:      label.NoLabel,
			wantFound: false,
		},
		{
			name:      "Second child does not find child directive",
			cfg:       secondChildCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/child/repo"},
			lang:      "go",
			want:      label.NoLabel,
			wantFound: false,
		},
		{
			name:      "Second child finds root directive",
			cfg:       secondChildCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/root/repo"},
			lang:      "go",
			want:      getTestLabel(t, "@com_example//root:replacement"),
			wantFound: true,
		},
		{
			name:       "Target resolves to label populated by regexp",
			cfg:        dualResolveRegexpCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/foo/foo_package"},
			lang:       "go",
			want:       getTestLabel(t, "@com_example//foo_package:replacement"),
			wantFound:  true,
		},
		{
			name:       "Target resolves to label populated by multiple captured regexp",
			cfg:        multipleExpDualResolveRegexpCfg,
			importSpec: ImportSpec{Lang: "go", Imp: "github.com/foo/foo_package/baz"},
			lang:       "go",
			want:       getTestLabel(t, "@com_example//foo_package/bar_sub_dir/baz:replacement"),
			wantFound:  true,
		},
		{
			name:       "Target resolves to label populated by the configuration with no captured regex",
			cfg:        nonCapturedRegexpCfg,
			importSpec: ImportSpec{Lang: "py", Imp: "github.com/root/repo"},
			lang:       "py",
			want:       getTestLabel(t, "@com_example//regexp:no_replacement"),
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FindRuleWithOverride(tt.cfg, tt.importSpec, tt.lang)
			if found != tt.wantFound {
				t.Fatalf("FindRuleWithOverride() found = %v, wantFound %v", found, tt.wantFound)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("FindRuleWithOverride() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func getConfig(t *testing.T, path string, directives []rule.Directive, parent *config.Config) *config.Config {
	cfg := &config.Config{
		Exts: map[string]interface{}{},
	}
	configurer := &Configurer{}
	configurer.RegisterFlags(nil, "", cfg)
	configurer.CheckFlags(nil, cfg)

	if parent != nil {
		cfg.Exts[resolveName] = parent.Exts[resolveName]
	}

	configurer.Configure(cfg, path, &rule.File{Directives: directives})
	return cfg
}

func getTestLabel(t *testing.T, str string) label.Label {
	l, err := label.Parse(str)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
