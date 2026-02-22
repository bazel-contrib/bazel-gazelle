TestGoImportVisibility checks that submodules implicitly declared with
go_repository rules in the repo config file (WORKSPACE) have visibility
for rules generated in internal directories where appropriate.
Verifies #619.
