//go:build go1.23
// +build go1.23

package mapkind

import (
	"iter"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

func allRules(f *rule.File, extraRuleLists ...[]*rule.Rule) iter.Seq2[int, *rule.Rule] {
	return func(yield func(int, *rule.Rule) bool) {
		idx := 0

		if f != nil {
			for _, r := range f.Rules {
				if !yield(idx, r) {
					return
				}
				idx++
			}
		}

		for _, extraRuleList := range extraRuleLists {
			for _, r := range extraRuleList {
				if !yield(idx, r) {
					return
				}
				idx++
			}
		}
	}
}
