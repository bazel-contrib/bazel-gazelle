package mapkind

import (
	"fmt"
	"log"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// ApplyOnRules returns all mapped kind rules of rules of the given file and of all the given rules list.
func ApplyOnRules(c *config.Config, knownKinds map[string]rule.KindInfo, f *rule.File, extraRuleLists ...[]*rule.Rule) (*MappedResult, error) {
	kindInfos := make(map[string]rule.KindInfo, len(knownKinds))
	for k, v := range knownKinds {
		kindInfos[k] = v
	}

	res := MappedResult{
		Kinds: kindInfos,
	}

	for _, r := range allRules(f, extraRuleLists...) {
		if err := applyOnRuleKind(&res, c, r); err != nil {
			return &res, err
		}

		if err := applyOnFirstRuleArg(&res, c, r); err != nil {
			return &res, err
		}
	}

	return &res, nil
}

// applyOnRuleKind applies all kind replacements on the kind of the given rule.
func applyOnRuleKind(res *MappedResult, c *config.Config, r *rule.Rule) error {
	ruleKind := r.Kind()

	repl, err := lookupReplacement(c.KindMap, ruleKind)
	if err != nil {
		return fmt.Errorf("looking up mapped kind: %v", err)
	}

	if repl != nil {
		r.SetKind(repl.KindName)

		res.add(repl, ruleKind)
	}

	return nil
}

// applyOnFirstRuleArg applies all kind replacements on the first argument of the fiven rule.
// This supports the maybe(java_library, ...) pattern, but checks only the first arg to
// avoids potential false positives from other uses of symbols.
func applyOnFirstRuleArg(res *MappedResult, c *config.Config, r *rule.Rule) error {
	ruleArgs := r.Args()

	if len(ruleArgs) == 0 {
		return nil
	}

	firstRuleArg := ruleArgs[0]

	ident, ok := firstRuleArg.(*build.Ident)
	if !ok {
		return nil
	}

	ruleKind := ident.Name

	// Don't allow re-mapping symbols that aren't known loads of a plugin.
	if _, found := res.Kinds[ruleKind]; !found {
		return nil
	}

	repl, err := lookupReplacement(c.KindMap, ruleKind)
	if err != nil {
		return fmt.Errorf("looking up mapped kind: %v", err)
	}

	if repl != nil {
		if err := r.UpdateArg(0, &build.Ident{Name: repl.KindName}); err != nil {
			log.Fatal(err) // Should never happen beacuse index 0 is always available.
		}

		res.add(repl, ruleKind)
	}

	return nil
}

// lookupReplacement finds a mapped replacement for rule kind `kind`, resolving transitively.
// i.e. if go_library is mapped to custom_go_library, and custom_go_library is mapped to other_go_library,
// looking up go_library will return other_go_library.
// It returns an error on a loop, and may return nil if no remapping should be performed.
func lookupReplacement(kindMap map[string]config.MappedKind, kind string) (*config.MappedKind, error) {
	var mapped *config.MappedKind
	seenKinds := make(map[string]struct{})
	seenKindPath := []string{kind}
	for {
		replacement, ok := kindMap[kind]
		if !ok {
			break
		}

		seenKindPath = append(seenKindPath, replacement.KindName)
		if _, alreadySeen := seenKinds[replacement.KindName]; alreadySeen {
			return nil, fmt.Errorf("found loop of map_kind replacements: %s", strings.Join(seenKindPath, " -> "))
		}

		seenKinds[replacement.KindName] = struct{}{}
		mapped = &replacement
		if kind == replacement.KindName {
			break
		}

		kind = replacement.KindName
	}

	return mapped, nil
}
