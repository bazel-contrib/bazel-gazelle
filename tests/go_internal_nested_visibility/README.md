This test checks how the Go extension generates visibility directives
for nested internal directories.

For `//internal`, visibility should be `//:__subpackages__`.

For `//a/internal`, visibility should be `//a:__subpackages__`.

For `//internal/a/internal/b`, visibility should be `//internal/a:__subpackages__`.
