package mapkind

import (
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// MappedResult provides information related to all mapped kind rules.
type MappedResult struct {
	// MappedKinds is the list of kind replacements used in all rules.
	MappedKinds []config.MappedKind

	// Kinds is the list of all known kind of rules, including all mapped kinds.
	Kinds map[string]rule.KindInfo
}

// add registers the given kind replacement used in a rule.
func (mr *MappedResult) add(mappedKind *config.MappedKind, ruleKind string) {
	var found bool
	for i := range mr.MappedKinds {
		k := &mr.MappedKinds[i]
		if k.FromKind == mappedKind.FromKind {
			k.KindLoad = mappedKind.KindLoad
			k.KindName = mappedKind.KindName
			found = true
			break
		}
	}

	if !found {
		mr.MappedKinds = append(mr.MappedKinds, *mappedKind)
	}

	if ki, ok := mr.Kinds[ruleKind]; ok {
		mr.Kinds[mappedKind.KindName] = ki
	}
}

// ApplyOnLoads applies all kind replacements on the given list of load instructions.
func (mr *MappedResult) ApplyOnLoads(loads []rule.LoadInfo) []rule.LoadInfo {
	if len(mr.MappedKinds) == 0 {
		return loads
	}

	// Add new RuleInfos or replace existing ones with merged ones.
	mappedLoads := make([]rule.LoadInfo, len(loads))
	copy(mappedLoads, loads)
	for _, mappedKind := range mr.MappedKinds {
		mappedLoads = appendOrMergeKindMapping(mappedLoads, mappedKind)
	}

	return mappedLoads
}

// appendOrMergeKindMapping adds LoadInfo for the given replacement.
func appendOrMergeKindMapping(mappedLoads []rule.LoadInfo, mappedKind config.MappedKind) []rule.LoadInfo {
	// If mappedKind.KindLoad already exists in the list, create a merged copy.
	for i, load := range mappedLoads {
		if load.Name == mappedKind.KindLoad {
			mappedLoads[i].Symbols = append(load.Symbols, mappedKind.KindName)
			return mappedLoads
		}
	}

	// Add a new LoadInfo.
	return append(mappedLoads, rule.LoadInfo{
		Name:    mappedKind.KindLoad,
		Symbols: []string{mappedKind.KindName},
	})
}
