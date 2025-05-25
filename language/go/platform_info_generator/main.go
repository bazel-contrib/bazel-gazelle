package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

func stringify(m map[string]struct{}) string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func quote(s string) string {
	return `"` + s + `"`
}

func main() {
	oss := make(map[string]struct{})
	archs := make(map[string]struct{})
	for _, p := range rule.KnownPlatforms {
		oss[quote(p.OS)] = struct{}{}
		archs[quote(p.Arch)] = struct{}{}
	}

	// Prevent `go generate` from picking this up.
	code := fmt.Sprintf("//" + `go:generate bazel run --run_under=cp //language/go:gen_platform_info.go ./platform_info.go

package golang

// These helpers are faster than map lookups using 'rule.KnownOSSet'/'rule.KnownArchSet'.

func IsKnownOS(os string) bool {
	switch os {
	case %s:
		return true
	default:
		return false
	}
}

func IsKnownArch(arch string) bool {
	switch arch {
	case %s:
		return true
	default:
		return false
	}
}
`, stringify(oss), stringify(archs))

	err := os.WriteFile(os.Args[1], []byte(code), 0600)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
