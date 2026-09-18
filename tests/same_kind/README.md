Verifies that two extensions can emit rules with the same kind names.
Gazelle should remember which extension generated a rule and should use
that extension's kind info when merging, indexing, and resolving rules.

Some ambiguity remains when indexing existing rules with the same kind name:
the last extension registered wins, arbitrarily.

It's also not really possible to load the same symbol from two different
files, so the last extension or the existing load wins.

This test uses a special Gazelle binary with two extensions:

- test_same_kind_1 generates r1. It fails if Imports or Resolve are called
  on a rule without match_attr_1.
- test_same_kind_2 generates r2 and indexes existing rule e2. It fails if
  Imports or Resolve are called on a rule without match_attr_2.
- There's no overlap in matchable, mergeable, or resolvable attributes,
  so it's also clear in the output file if the wrong extension was called.
