// Package collision_mangle is one half of a pair of fixture modules whose import
// paths ("example.org/collision/mangle" and "example.org/collision_mangle")
// mangle to the same default Bazel repo name (org_example_collision_mangle). It
// exercises the module_override "repo_name" attribute, which lets the root
// module assign a distinct repo name to one of them so both can coexist.
package collision_mangle

const Name = "underscore"
