//go:build !go1.23
// +build !go1.23

package mapkind

import (
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func allRules(f *rule.File, extraRuleLists ...[]*rule.Rule) []*rule.Rule {
	count := 0
	if f != nil {
		count += len(f.Rules)
	}
	for _, extraRuleList := range extraRuleLists {
		count += len(extraRuleList)
	}

	rules := make([]*rule.Rule, 0, count)
	if f != nil {
		rules = append(rules, f.Rules...)
	}
	for _, extraRuleList := range extraRuleLists {
		rules = append(rules, extraRuleList...)
	}

	return rules
}
