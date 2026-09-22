# Gazelle `-strict` mode

When Gazelle is run with the `-strict` flag, it logs errors as it encounters
them but continues updating other directories when possible. After a full run,
Gazelle exits with a non-zero status if any errors occurred (for example, BUILD
file syntax errors or unknown directives).

Without `-strict`, Gazelle behaves the same way but exits zero even when errors
were logged.

This test has syntax errors in `a/` and `b/`. It expects both errors on stderr
and exit code 1.
