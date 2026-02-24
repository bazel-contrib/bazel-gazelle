# go_minimal_module_compatibility_aliases

TestMinimalModuleCompatibilityAliases checks that importpath_aliases
are emitted for go_libraries when needed. This can't easily be checked
in language/go because the generator tests don't support running at
the repository root or with additional flags, both of which are required.
