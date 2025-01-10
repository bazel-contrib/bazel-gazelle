Gazelle build file generator
============================

.. All external links are here
.. _a gazelle extension: https://github.com/bazel-contrib/rules_jvm/tree/main/java/gazelle
.. _Kotlin Support: https://github.com/aspect-build/aspect-cli/blob/main/gazelle/kotlin/
.. _JavaScript and TypeScript Support: https://github.com/aspect-build/aspect-cli/blob/main/gazelle/js/
.. _Architecture of Gazelle: Design.rst
.. _Repository rules: repository.md
.. _go_repository: repository.md#go_repository
.. _fix: #fix-and-update
.. _update: #fix-and-update
.. _Avoiding conflicts with proto rules: https://github.com/bazelbuild/rules_go/blob/master/proto/core.rst#avoiding-conflicts
.. _gazelle rule: #bazel-rule
.. _doublestar.Match: https://github.com/bmatcuk/doublestar#match
.. _Extending Gazelle: extend.md
.. _extensions.md: extensions.md#go_deps
.. _Go Bzlmod docs: https://github.com/bazel-contrib/rules_go/blob/master/docs/go/core/bzlmod.md
.. _extended: `Extending Gazelle`_
.. _gazelle_binary: extend.md#gazelle_binary
.. _import_prefix: https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.import_prefix
.. _strip_import_prefix: https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.strip_import_prefix
.. _buildozer: https://github.com/bazelbuild/buildtools/tree/master/buildozer
.. _Go Release Policy: https://golang.org/doc/devel/release.html#policy
.. _bazel-go-discuss: https://groups.google.com/forum/#!forum/bazel-go-discuss
.. _#bazel on Go Slack: https://gophers.slack.com/archives/C1SCQE54N
.. _#go on Bazel Slack: https://bazelbuild.slack.com/archives/CDBP88Z0D
.. _#514: https://github.com/bazelbuild/rules_python/pull/514
.. _#1030: https://github.com/bazelbuild/bazel-gazelle/issues/1030
.. _rules_jvm: https://github.com/bazel-contrib/rules_jvm
.. _rules_python: https://github.com/bazelbuild/rules_python
.. _rules_r: https://github.com/grailbio/rules_r
.. _rules_haskell: https://github.com/tweag/rules_haskell
.. _rules_nodejs_gazelle: https://github.com/benchsci/rules_nodejs_gazelle
.. _bazel-skylib: https://github.com/bazelbuild/bazel-skylib
.. _bazel_skylib/gazelle/bzl: https://github.com/bazelbuild/bazel-skylib/tree/master/gazelle/bzl
.. _gazelle_cabal: https://github.com/tweag/gazelle_cabal
.. _gazelle_haskell_modules: https://github.com/tweag/gazelle_haskell_modules
.. _stackb/rules_proto: https://github.com/stackb/rules_proto
.. _Open a PR: https://github.com/bazelbuild/bazel-gazelle/edit/master/README.rst
.. _Bazel Slack: https://slack.bazel.build
.. _rules_swift_package_manager: https://github.com/cgrindel/rules_swift_package_manager
.. _gazelle_rust: https://github.com/Calsign/gazelle_rust
.. _rules_rust: https://github.com/bazelbuild/rules_rust
.. _Verbs Tutorial: https://bazel.build/rules/verbs-tutorial

.. role:: cmd(code)
.. role:: flag(code)
.. role:: direc(code)
.. role:: kbd
.. role:: param(kbd)
.. role:: type(emphasis)
.. role:: value(code)
.. |mandatory| replace:: **mandatory value**
.. End of directives

Gazelle is a build file generator for Bazel projects. It can create new
BUILD.bazel files for a project that follows language conventions, and it can
update existing build files to include new sources, dependencies, and
options. Gazelle natively supports Go and protobuf, and it may be extended_
to support new languages and custom rule sets.

Gazelle may be run by Bazel using the `gazelle rule`_, or it may be installed
and run as a command line tool. Gazelle can also generate build files for
external repositories as part of the `go_repository`_ rule.

*Gazelle is under active development. Its interface and the rules it generates
may change. Gazelle is not an official Google product.*

Mailing list: `bazel-go-discuss`_

Slack: `#go on Bazel Slack`_, `#bazel on Go Slack`_

*rules_go and Gazelle are getting community maintainers! If you are a regular
user of either project and are interested in helping out with development,
code reviews, and issue triage, please drop by our Slack channels (linked above)
and say hello!*

.. contents:: **Contents**
  :depth: 2

**See also:**

* `Architecture of Gazelle`_
* `Repository rules`_

  * `go_repository`_

* `Extending Gazelle`_
* `Avoiding conflicts with proto rules`_

Supported languages
-------------------

Gazelle can generate Bazel BUILD files for many languages:

* Go

  Go supported is included here in bazel-gazelle, see below.

* Haskell

  Tweag's `rules_haskell`_ has two extensions: `gazelle_cabal`_, for generating rules from Cabal files
  and `gazelle_haskell_modules`_ for even more fine-grained build definitions.

* Java

  bazel-contrib's `rules_jvm`_ extensions include `a gazelle extension`_ for
  generating ``java_library``, ``java_binary``, ``java_test``, and ``java_test_suite`` rules.

* JavaScript / TypeScript

  Aspect provides `JavaScript and TypeScript Support`_ in aspect-cli (also usable separately).

  BenchSci's `rules_nodejs_gazelle`_ supports generating `ts_project`, `js_library`, `jest_test`,
  and `web_asset` rules, and is able to support module bundlers like Webpack and Next.js


* Kotlin

  Aspect Build provides some `Kotlin Support`_ in the repo of their aspect-cli (also usable separately).
  Still under development, please check the README for currently available features.

* Protocol Buffers

  Support for the `proto_library` rule, as well as `go_proto_library` is in this repository, see below.
  Other language-specific proto rules are not supported here.
  `stackb/rules_proto`_ is a good resource for these rules.

* Python

  `rules_python`_ has an extension for generating ``py_library``, ``py_binary``, and ``py_test`` rules.

* R

  `rules_r`_ has an extension for generating rules for R package builds and tests.

* Rust

  `gazelle_rust`_ is an extension for generating `rules_rust`_ targets.

* Starlark

  `bazel-skylib`_ has an extension for generating ``bzl_library`` rules. See `bazel_skylib/gazelle/bzl`_.

* Swift

  `rules_swift_package_manager`_ has an extension for generating ``swift_library``, ``swift_binary``, and
  ``swift_test`` rules. It also includes facilities for resolving, downloading and building external Swift
  packages for a Bazel workspace.

If you know of an extension which could be linked here, please `open a PR`_!

More languages can be added by `Extending Gazelle`_.
Chat with us in the ``#gazelle`` channel on `Bazel Slack`_ if you'd like to discuss your design.

If you've written your own extension, please consider open-sourcing it for
use by the rest of the community.
Note that such extensions belong in a language-specific repository, not in bazel-gazelle.
See discussion in `#1030`_.

Setup
-----

Bzlmod
~~~~~~

See the `Go Bzlmod docs`_.

The full documentation for the ``go_deps`` extension is in `extensions.md`_.

WORKSPACE
~~~~~~~~~

To use Gazelle in a new project, add the ``bazel_gazelle`` repository and its
dependencies to your WORKSPACE file and call ``gazelle_dependencies``. It
should look like this:

.. code:: bzl

    load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

    http_archive(
        name = "io_bazel_rules_go",
        integrity = "sha256-M6zErg9wUC20uJPJ/B3Xqb+ZjCPn/yxFF3QdQEmpdvg=",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
        ],
    )

    http_archive(
        name = "bazel_gazelle",
        integrity = "sha256-12v3pg/YsFBEQJDfooN6Tq+YKeEWVhjuNdzspcvfWNU=",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
        ],
    )


    load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
    load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

    ############################################################
    # Define your own dependencies here using go_repository.
    # Else, dependencies declared by rules_go/gazelle will be used.
    # The first declaration of an external repository "wins".
    ############################################################

    go_rules_dependencies()

    go_register_toolchains(version = "1.20.5")

    gazelle_dependencies()

``gazelle_dependencies`` supports optional argument ``go_env`` (dict-mapping)
to set project specific go environment variables. If you are using a
`WORKSPACE.bazel` file, you will need to specify that using:

.. code:: bzl

    gazelle_dependencies(go_repository_default_config = "//:WORKSPACE.bazel")

Add the code below to the BUILD or BUILD.bazel file in the root directory
of your repository.

**Important:** For Go projects, replace the string after ``prefix`` with
the portion of your import path that corresponds to your repository.

.. code:: bzl

  load("@bazel_gazelle//:def.bzl", "gazelle")

  # gazelle:prefix github.com/example/project
  gazelle(name = "gazelle")

After adding this code, you can run Gazelle with Bazel.

.. code::

  $ bazel run //:gazelle

This will generate new BUILD.bazel files for your project. You can run the same
command in the future to update existing BUILD.bazel files to include new source
files or options.

You can write other ``gazelle`` rules to run alternate commands like ``update-repos``.

.. code:: bzl

  gazelle(
      name = "gazelle-update-repos",
      args = [
          "-from_file=go.mod",
          "-to_macro=deps.bzl%go_dependencies",
          "-prune",
      ],
      command = "update-repos",
  )

You can also pass additional arguments to Gazelle after a ``--`` argument.

.. code::

  $ bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=deps.bzl%go_dependencies

After running ``update-repos``, you might want to run ``bazel run //:gazelle`` again, as the
``update-repos`` command can affect the output of a normal run of Gazelle.

To verify that all BUILD files are update-to-date, you can use the ``gazelle_test`` rule.

.. code:: bzl

  load("@bazel_gazelle//:def.bzl", "gazelle_test")

  gazelle_test(
      name = "gazelle_test",
      workspace = "//:BUILD.bazel", # a file in the workspace root, where the gazelle will be run
  )

However, please note that gazelle_test cannot be cached.

Running Gazelle with Go
~~~~~~~~~~~~~~~~~~~~~~~

If you have a Go toolchain installed, you can install Gazelle with the
command below:

.. code::

  go install github.com/bazelbuild/bazel-gazelle/cmd/gazelle@latest

Make sure to re-run this command to upgrade Gazelle whenever you upgrade
rules_go in your repository.

To generate BUILD.bazel files in a new project, run the command below, replacing
the prefix with the portion of your import path that corresponds to your
repository.

.. code::

  gazelle -go_prefix github.com/example/project

Most of Gazelle's command-line arguments can be expressed as special comments
in build files. See Directives_ below. You may want to copy this line into
your root build files to avoid having to type ``-go_prefix`` every time.

.. code:: bzl

  # gazelle:prefix github.com/example/project

Compatibility with Go
---------------------

Gazelle is compatible with supported releases of Go, per the
`Go Release Policy`_. The Go Team officially supports the current and previous
minor releases. Older releases are not supported and don't receive bug fixes
or security updates.

Gazelle may use language and library features from the oldest supported release.

Compatibility with rules_go
---------------------------

Gazelle generates build files that use features in newer versions of
``rules_go``. Newer versions of Gazelle *may* generate build files that work
with older versions of ``rules_go``, but check the table below to ensure
you're using a compatible version.

+---------------------+------------------------------+------------------------------+
| **Gazelle version** | **Minimum rules_go version** | **Maximum rules_go version** |
+=====================+==============================+==============================+
| 0.8                 | 0.8                          | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.9                 | 0.9                          | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.10                | 0.9                          | 0.11                         |
+---------------------+------------------------------+------------------------------+
| 0.11                | 0.11                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.12                | 0.11                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.13                | 0.13                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.14                | 0.13                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.15                | 0.13                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.16                | 0.13                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.17                | 0.13                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.18                | 0.19                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.19                | 0.19                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.20                | 0.20                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.21                | 0.20                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.22                | 0.20                         | 0.24                         |
+---------------------+------------------------------+------------------------------+
| 0.23                | 0.26                         | 0.28                         |
+---------------------+------------------------------+------------------------------+
| 0.24                | 0.29                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.25                | 0.29                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.26                | 0.29                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.27                | 0.29                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.28                | 0.35                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.29                | 0.35                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.30                | 0.35                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.31                | 0.35                         | 0.40                         |
+---------------------+------------------------------+------------------------------+
| 0.32                | **0.41**                     | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.33                | 0.41                         | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.34                | 0.41                         | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.35                | 0.41                         | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.36                | 0.41                         | n/a                          |
+---------------------+------------------------------+------------------------------+
| 0.37                | 0.41                         | n/a                          |
+---------------------+------------------------------+------------------------------+

Usage
-----

Command line
~~~~~~~~~~~~

.. code::

  gazelle <command> [flags...] [package-dirs...]

The first argument to Gazelle may be one of the commands below. If no command
is specified, ``update`` is assumed. The remaining arguments are specific
to each command and are documented below.

update_
  Scans sources files, then generates and updates build files.

fix_
  Same as the ``update`` command, but it also fixes deprecated usage of rules.

update-repos_
  Adds and updates repository rules in the WORKSPACE file.

Bazel rule
~~~~~~~~~~

Gazelle may be run via a rule. See `Running Gazelle with Bazel`_ for setup
instructions. This rule builds Gazelle and generates a wrapper script that
executes Gazelle with baked-in set of arguments. You can run this script
with ``bazel run``, or you can copy it into your workspace and run it directly.

The following attributes are available on the ``gazelle`` rule.

+----------------------+---------------------+--------------------------------------+
| **Name**             | **Type**            | **Default value**                    |
+======================+=====================+======================================+
| :param:`gazelle`     | :type:`label`       | :value:`@bazel_gazelle//cmd/gazelle` |
+----------------------+---------------------+--------------------------------------+
| The `gazelle_binary`_ rule that builds Gazelle. You can substitute a modified     |
| version of Gazelle with this. See `Extending Gazelle`_.                           |
+----------------------+---------------------+--------------------------------------+
| :param:`external`    | :type:`string`      | :value:`external`                    |
+----------------------+---------------------+--------------------------------------+
| The method for resolving unknown imports to Bazel dependencies. May be            |
| :value:`external`, :value:`static` or :value:`vendored`.                          |
| See `Dependency resolution`_.                                                     |
+----------------------+---------------------+--------------------------------------+
| :param:`build_tags`  | :type:`string_list` | :value:`[]`                          |
+----------------------+---------------------+--------------------------------------+
| The list of Go build tags that Gazelle should consider to always be true.         |
+----------------------+---------------------+--------------------------------------+
| :param:`prefix`      | :type:`string`      | :value:`""`                          |
+----------------------+---------------------+--------------------------------------+
| The import path that corresponds to the repository root directory.                |
|                                                                                   |
| Note: It's usually better to write a directive like                               |
| ``# gazelle:prefix example.com/repo`` in your build file instead of setting       |
| this attribute.                                                                   |
+----------------------+---------------------+--------------------------------------+
| :param:`extra_args`  | :type:`string_list` | :value:`[]`                          |
+----------------------+---------------------+--------------------------------------+
| A list of extra command line arguments passed to Gazelle.  Note that              |
| ``extra_args`` are suppressed by extra command line args (e.g.                    |
| ``bazel run //:gazelle -- subdir``).                                              |
| See https://github.com/bazelbuild/bazel-gazelle/issues/536 for explanation.       |
+----------------------+---------------------+--------------------------------------+
| :param:`command`     | :type:`string`      | :value:`update`                      |
+----------------------+---------------------+--------------------------------------+
| The Gazelle command to use. May be :value:`fix`, :value:`update` or               |
| :value:`update-repos`.                                                            |
+----------------------+---------------------+--------------------------------------+

``fix`` and ``update``
~~~~~~~~~~~~~~~~~~~~~~

The ``update`` command is the most common way of running Gazelle. Gazelle
scans sources in directories throughout the repository, then creates and updates
build files.

The ``fix`` command does everything ``update`` does, but it also fixes
deprecated usage of rules, analogous to ``go fix``. For example, ``cgo_library``
will be consolidated with ``go_library``. This command may delete or rename
rules, so it's not on by default. See `Fix command transformations`_
for details.

Both commands accept a list of directories to process as positional arguments.
If no directories are specified, Gazelle will process the current directory.
Subdirectories will be processed recursively.

The following flags are accepted:

+-------------------------------------------------------------------+----------------------------------------+
| **Name**                                                          | **Default value**                      |
+===================================================================+========================================+
| :flag:`-build_file_name file1,file2,...`                          | :value:`BUILD.bazel,BUILD`             |
+-------------------------------------------------------------------+----------------------------------------+
| Comma-separated list of file names. Gazelle recognizes these files as Bazel                                |
| build files. New files will use the first name in this list. Use this if                                   |
| your project contains non-Bazel files named ``BUILD`` (or ``build`` on                                     |
| case-insensitive file systems).                                                                            |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-build_tags tag1,tag2`                                     |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| List of Go build tags Gazelle will consider to be true. Gazelle applies                                    |
| constraints when generating Go rules. It assumes certain tags are true on                                  |
| certain platforms (for example, ``amd64,linux``). It assumes all Go release                                |
| tags are true (for example, ``go1.8``). It considers other tags to be false                                |
| (for example, ``ignore``). This flag overrides that behavior.                                              |
|                                                                                                            |
| Bazel may still filter sources with these tags. Use                                                        |
| ``bazel build --define gotags=foo,bar`` to set tags at build time.                                         |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-exclude pattern`                                          |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| Prevents Gazelle from processing a file or directory if the given                                          |
| `doublestar.Match`_ pattern matches. If the pattern refers to a source file,                               |
| Gazelle won't include it in any rules. If the pattern refers to a directory,                               |
| Gazelle won't recurse into it.                                                                             |
|                                                                                                            |
| This option may be repeated. Patterns must be slash-separated, relative to the                             |
| repository root. This is equivalent to the ``# gazelle:exclude pattern``                                   |
| directive.                                                                                                 |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-external external|static|vendored`                        | :value:`external`                      |
+-------------------------------------------------------------------+----------------------------------------+
| Determines how Gazelle resolves import paths that cannot be resolve in the                                 |
| current repository. May be :value:`external`, :value:`static` or :value:`vendored`. See                    |
| `Dependency resolution`_.                                                                                  |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-index true|false`                                         | :value:`true`                          |
+-------------------------------------------------------------------+----------------------------------------+
| Determines whether Gazelle should index the libraries in the current repository and whether it             |
| should use the index to resolve dependencies. If this is switched off, Gazelle would rely on               |
| ``# gazelle:prefix`` directive or ``-go_prefix`` flag to resolve dependencies.                             |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-go_grpc_compiler`                                         | ``@io_bazel_rules_go//proto:go_grpc``  |
+-------------------------------------------------------------------+----------------------------------------+
| The protocol buffers compiler to use for building go bindings for gRPC. May be repeated.                   |
|                                                                                                            |
| See `Predefined plugins`_ for available options; commonly used options include                             |
| ``@io_bazel_rules_go//proto:gofast_grpc`` and ``@io_bazel_rules_go//proto:gogofaster_grpc``.               |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-go_naming_convention`                                     |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| Controls the names of generated Go targets. Equivalent to the                                              |
| ``# gazelle:go_naming_convention`` directive. See details in                                               |
| `Directives`_ below.                                                                                       |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-go_naming_convention_external`                            |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| Controls the default naming convention used when resolving libraries in                                    |
| external repositories with unknown naming conventions. Equivalent to the                                   |
| ``# gazelle:go_naming_convention_external`` directive.                                                     |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-go_prefix example.com/repo`                               |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| A prefix of import paths for libraries in the repository that corresponds to                               |
| the repository root. Equivalent to setting the ``# gazelle:prefix`` directive                              |
| in the root BUILD.bazel file or the ``prefix`` attribute of the ``gazelle`` rule. If                       |
| neither of those are set, this option is mandatory.                                                        |
|                                                                                                            |
| This prefix is used to determine whether an import path refers to a library                                |
| in the current repository or an external dependency.                                                       |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-go_proto_compiler`                                        | ``@io_bazel_rules_go//proto:go_proto`` |
+-------------------------------------------------------------------+----------------------------------------+
| The protocol buffers compiler to use for building go bindings. May be repeated.                            |
|                                                                                                            |
| See `Predefined plugins`_ for available options; commonly used options include                             |
| ``@io_bazel_rules_go//proto:gofast_proto`` and ``@io_bazel_rules_go//proto:gogofaster_proto``.             |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-known_import example.com`                                 |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| Skips import path resolution for a known domain. May be repeated.                                          |
|                                                                                                            |
| When Gazelle resolves an import path to an external dependency, it attempts                                |
| to discover the remote repository root over HTTP. Gazelle skips this                                       |
| discovery step for a few well-known domains with predictable structure, like                               |
| golang.org and github.com. This flag specifies additional domains to skip,                                 |
| which is useful in situations where the lookup would fail for some reason.                                 |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-mode fix|print|diff`                                      | :value:`fix`                           |
+-------------------------------------------------------------------+----------------------------------------+
| Method for emitting merged build files.                                                                    |
|                                                                                                            |
| In ``fix`` mode, Gazelle writes generated and merged files to disk. In                                     |
| ``print`` mode, it prints them to stdout. In ``diff`` mode, it prints a                                    |
| unified diff.                                                                                              |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-proto default|file|package|legacy|disable|disable_global` | :value:`default`                       |
+-------------------------------------------------------------------+----------------------------------------+
| Determines how Gazelle should generate rules for .proto files. See details                                 |
| in `Directives`_ below.                                                                                    |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-proto_group group`                                        | :value:`""`                            |
+-------------------------------------------------------------------+----------------------------------------+
| Determines the proto option Gazelle uses to group .proto files into rules                                  |
| when in ``package`` mode. See details in `Directives`_ below.                                              |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-proto_import_prefix path`                                 |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| Sets the `import_prefix`_ attribute of generated ``proto_library`` rules.                                  |
| This adds a prefix to the string used to import ``.proto`` files listed in                                 |
| the ``srcs`` attribute of generated rules. Equivalent to the                                               |
| ``# gazelle:proto_import_prefix`` directive. See details in `Directives`_ below.                           |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-repo_root dir`                                            |                                        |
+-------------------------------------------------------------------+----------------------------------------+
| The root directory of the repository. Gazelle normally infers this to be the                               |
| directory containing the WORKSPACE file.                                                                   |
|                                                                                                            |
| Gazelle will not process packages outside this directory.                                                  |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-lang lang1,lang2,...`                                     | :value:`""`                            |
+-------------------------------------------------------------------+----------------------------------------+
| Selects languages for which to compose and index rules.                                                    |
|                                                                                                            |
| By default, all languages that this Gazelle was built with are processed.                                  |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-cpuprofile filename`                                      | :value:`""`                            |
+-------------------------------------------------------------------+----------------------------------------+
| If specified, gazelle uses [runtime/pprof](https://pkg.go.dev/runtime/pprof#StartCPUProfile) to collect    |
| CPU profiling information from the command and save it to the given file.                                  |
|                                                                                                            |
| By default, this is disabled                                                                               |
+-------------------------------------------------------------------+----------------------------------------+
| :flag:`-memprofile filename`                                      | :value:`""`                            |
+-------------------------------------------------------------------+----------------------------------------+
| If specified, gazelle uses [runtime/pprof](https://pkg.go.dev/runtime/pprof#WriteHeapProfile) to collect   |
| memory a profile information from the command and save it to a file.                                       |
|                                                                                                            |
| By default, this is disabled                                                                               |
+-------------------------------------------------------------------+----------------------------------------+

.. _Predefined plugins: https://github.com/bazelbuild/rules_go/blob/master/proto/core.rst#predefined-plugins

``update-repos``
~~~~~~~~~~~~~~~~

The ``update-repos`` command updates repository rules.  It can write the rules
to either the WORKSPACE (by default) or a .bzl file macro function.  It can be
used to add new repository rules or update existing rules to the specified
version. It can also import repository rules from a ``go.mod`` or a ``go.work``
file.

WARNING: This command is mainly used for managing external Go dependencies in Bazel's WORKSPACE mode.
For managing external Go dependencies in Bazel's BzlMod mode, please check: https://github.com/bazelbuild/rules_go/blob/master/docs/go/core/bzlmod.md#external-dependencies

.. code:: bash

  # Add or update a repository to latest version by import path
  $ gazelle update-repos example.com/new/repo

  # Add or update a repository to specified version/commit by import path
  $ gazelle update-repos example.com/new/repo@v1.3.1

  # Import repositories from go.mod
  $ gazelle update-repos -from_file=go.mod

  # Import repositories from go.work
  $ gazelle update-repos -from_file=go.work

  # Import repositories from go.mod and update macro
  $ gazelle update-repos -from_file=go.mod -to_macro=repositories.bzl%go_repositories

  # Import repositories from go.work and update macro
  $ gazelle update-repos -from_file=go.work -to_macro=repositories.bzl%go_repositories

The following flags are accepted:

+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| **Name**                                                                                                 | **Default value**                            |
+==========================================================================================================+==============================================+
| :flag:`-from_file lock-file`                                                                             |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Import repositories from a file as `go_repository`_ rules. These rules will be added to the bottom of the WORKSPACE file or merged with existing rules. |
|                                                                                                                                                         |
| The lock file format is inferred from the file name. ``go.mod`` and ``go.work`` are all supported.                                                      |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-repo_root dir`                                                                                   |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| The root directory of the repository. Gazelle normally infers this to be the directory containing the WORKSPACE file.                                   |
|                                                                                                                                                         |
| Gazelle will not process packages outside this directory.                                                                                               |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-to_macro macroFile%defName`                                                                      |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Tells Gazelle to write new repository rules into a .bzl macro function rather than the WORKSPACE file.                                                  |
|                                                                                                                                                         |
| The ``repository_macro`` directive should be added to the WORKSPACE in order for future Gazelle calls to recognize the repos defined in the macro file. |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-prune true|false`                                                                                | :value:`false`                               |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| When true, Gazelle will remove `go_repository`_ rules that no longer have equivalent repos in the ``go.mod`` file.                                      |
|                                                                                                                                                         |
| This flag can only be used with ``-from_file``.                                                                                                         |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_directives arg1,arg2,...`                                                                  |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_directives attribute`` for the generated `go_repository`_ rule(s).                                                                     |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_external external|vendored`                                                                |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_external`` attribute for the generated `go_repository`_ rule(s).                                                                       |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_extra_args arg1,arg2,...`                                                                  |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_extra_args attribute`` for the generated `go_repository`_ rule(s).                                                                     |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_file_generation auto|on|off|clean`                                                         |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_file_generation`` attribute for the generated `go_repository`_ rule(s).                                                                |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_file_names file1,file2,...`                                                                |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_file_name`` attribute for the generated `go_repository`_ rule(s).                                                                      |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_file_proto_mode default|package|legacy|disable|disable_global`                             |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_file_proto_mode`` attribute for the generated `go_repository`_ rule(s).                                                                |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| :flag:`-build_tags tag1,tag2,...`                                                                        |                                              |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+
| Sets the ``build_tags`` attribute for the generated `go_repository`_ rule(s).                                                                           |
+----------------------------------------------------------------------------------------------------------+----------------------------------------------+

Directives
~~~~~~~~~~

Gazelle can be configured with *directives*, which are written as top-level
comments in build files. Most options that can be set on the command line
can also be set using directives. Some options can only be set with
directives.

Directive comments have the form ``# gazelle:key value``. For example:

.. code:: bzl

  load("@io_bazel_rules_go//go:def.bzl", "go_library")

  # gazelle:prefix github.com/example/project
  # gazelle:build_file_name BUILD,BUILD.bazel

  go_library(
      name = "go_default_library",
      srcs = ["example.go"],
      importpath = "github.com/example/project",
      visibility = ["//visibility:public"],
  )

Directives apply in the directory where they are set *and* in subdirectories.
This means, for example, if you set ``# gazelle:prefix`` in the build file
in your project's root directory, it affects your whole project. If you
set it in a subdirectory, it only affects rules in that subtree.

The following directives are recognized:

+---------------------------------------------------+----------------------------------------+
| **Directive**                                     | **Default value**                      |
+===================================================+========================================+
| :direc:`# gazelle:build_file_name names`          | :value:`BUILD.bazel,BUILD`             |
+---------------------------------------------------+----------------------------------------+
| Comma-separated list of file names. Gazelle recognizes these files as Bazel                |
| build files. New files will use the first name in this list. Use this if                   |
| your project contains non-Bazel files named ``BUILD`` (or ``build`` on                     |
| case-insensitive file systems).                                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:build_tags foo,bar`             | none                                   |
+---------------------------------------------------+----------------------------------------+
| List of Go build tags Gazelle will consider to be true. Gazelle applies                    |
| constraints when generating Go rules. It assumes certain tags are true on                  |
| certain platforms (for example, ``amd64,linux``). It assumes all Go release                |
| tags are true (for example, ``go1.8``). It considers other tags to be false                |
| (for example, ``ignore``). This flag overrides that behavior.                              |
|                                                                                            |
| Bazel may still filter sources with these tags. Use                                        |
| ``bazel build --define gotags=foo,bar`` to set tags at build time.                         |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:exclude pattern`                | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Prevents Gazelle from processing a file or directory if the given                          |
| `doublestar.Match`_ pattern matches. If the pattern refers to a source file,               |
| Gazelle won't include it in any rules. If the pattern refers to a directory,               |
| Gazelle won't recurse into it. This directive may be repeated to exclude                   |
| multiple patterns, one per line.                                                           |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:follow pattern`                 | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Instructs Gazelle to follow a symbolic link to a directory within the repository if the    |
| given `doublestar.Match`_ pattern matches. Normally, Gazelle does not follow symbolic      |
| links unless they point outside of the repository root.                                    |
|                                                                                            |
| Care must be taken to avoid visiting a directory more than once.                           |
| The ``# gazelle:exclude`` directive may be used to prevent Gazelle from                    |
| recursing into a directory.                                                                |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_generate_proto`              | ``true``                               |
+---------------------------------------------------+----------------------------------------+
| Instructs Gazelle's Go extension whether to generate ``go_proto_library`` rules for        |
| ``proto_library`` rules generated by the Proto extension. When this directive is ``true``  |
| Gazelle will generate ``go_proto_library`` and ``go_library`` according to                 |
| ``# gazelle:proto``. When this directive is ``false``, the Go extension will ignore any    |
| ``proto_library`` rules. If there are any pre-generated Go files, they will be treated as  |
| regular Go files.                                                                          |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_test mode`                   | ``default``                            |
+---------------------------------------------------+----------------------------------------+
| Tells Gazelle how to generate rules for _test.go files. Valid values are:                  |
|                                                                                            |
| * ``default``: One ``go_test`` rule will be generated whose ``srcs`` includes              |
|   all ``_test.go`` files in the directory.                                                 |
| * ``file``: A distinct ``go_test`` rule will be generated for each ``_test.go`` file in the|
|   package directory.                                                                       |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_grpc_compilers`              | ``@io_bazel_rules_go//proto:go_grpc``  |
+---------------------------------------------------+----------------------------------------+
| The protocol buffers compiler(s) to use for building go bindings for gRPC.                 |
| Multiple compilers, separated by commas, may be specified.                                 |
| Omit the directive value to reset ``go_grpc_compilers`` back to the default.               |
|                                                                                            |
| See `Predefined plugins`_ for available options; commonly used options include             |
| ``@io_bazel_rules_go//proto:gofast_grpc`` and                                              |
| ``@io_bazel_rules_go//proto:gogofaster_grpc``.                                             |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_naming_convention`           | inferred automatically                 |
+---------------------------------------------------+----------------------------------------+
| Controls the names of generated Go targets.                                                |
|                                                                                            |
| Valid values are:                                                                          |
|                                                                                            |
| * ``go_default_library``: Library targets are named ``go_default_library``, test targets   |
|   are named ``go_default_test``.                                                           |
| * ``import``: Library and test targets are named after the last segment of their import    |
|   path.                                                                                    |
|   For example, ``example.repo/foo`` is named ``foo``, and the test target is ``foo_test``. |
|   Major version suffixes like ``/v2`` are dropped.                                         |
|   For a main package with a binary ``foobin``, the names are instead ``foobin_lib`` and    |
|   ``foobin_test``.                                                                         |
| * ``import_alias``: Same as ``import``, but an ``alias`` target is generated named         |
|   ``go_default_library`` to ensure backwards compatibility.                                |
|                                                                                            |
| If no naming convention is set, Gazelle attempts to infer the convention in                |
| use by reading the root build file and build files in immediate                            |
| subdirectories. If no Go targets are found, Gazelle defaults to ``import``.                |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_naming_convention_external`  | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Controls the default naming convention used when resolving libraries in                    |
| external repositories with unknown naming conventions. Accepts the same values             |
| as ``go_naming_convention``.                                                               |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_proto_compilers`             | ``@io_bazel_rules_go//proto:go_proto`` |
+---------------------------------------------------+----------------------------------------+
| The protocol buffers compiler(s) to use for building go bindings.                          |
| Multiple compilers, separated by commas, may be specified.                                 |
| Omit the directive value to reset ``go_proto_compilers`` back to the default.              |
|                                                                                            |
| See `Predefined plugins`_ for available options; commonly used options include             |
| ``@io_bazel_rules_go//proto:gofast_proto`` and                                             |
| ``@io_bazel_rules_go//proto:gogofaster_proto``.                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:ignore`                         | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Prevents Gazelle from modifying the build file. Gazelle will still read                    |
| rules in the build file and may modify build files in subdirectories.                      |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:importmap_prefix path`          | See below                              |
+---------------------------------------------------+----------------------------------------+
| A prefix for ``importmap`` attributes in library rules. Gazelle will set                   |
| an ``importmap`` on a ``go_library`` or ``go_proto_library`` by                            |
| concatenating this with the relative path from the directory where the                     |
| prefix is set to the library. For example, if ``importmap_prefix`` is set                  |
| to ``"x/example.com/repo"`` in the build file ``//foo/bar:BUILD.bazel``,                   |
| then a library in ``foo/bar/baz`` will have the ``importmap`` of                           |
| ``"x/example.com/repo/baz"``.                                                              |
|                                                                                            |
| ``importmap`` is not set when it matches ``importpath``.                                   |
|                                                                                            |
| As a special case, when Gazelle enters a directory named ``vendor``, it                    |
| sets ``importmap_prefix`` to a string based on the repository name and the                 |
| location of the vendor directory. If you wish to override this, you'll need                |
| to set ``importmap_prefix`` explicitly in the vendor directory.                            |
+------------------------------------------------------------+-------------------------------+
| :direc:`# gazelle:map_kind from_kind to_kind to_kind_load` | n/a                           |
+------------------------------------------------------------+-------------------------------+
| Customizes the kind of rules generated by Gazelle.                                         |
|                                                                                            |
| As a separate step after generating rules, any new rules of kind ``from_kind`` have their  |
| kind replaced with ``to_kind``. This means that ``to_kind`` must accept the same           |
| parameters and behave similarly.                                                           |
|                                                                                            |
| Most commonly, this would be used to replace the rules provided by ``rules_go`` with       |
| custom macros. For example,                                                                |
| ``gazelle:map_kind go_binary go_deployable //tools/go:def.bzl`` would configure Gazelle to |
| produce rules of kind ``go_deployable`` as loaded from ``//tools/go:def.bzl`` instead of   |
| ``go_binary``, for this directory or within.                                               |
|                                                                                            |
| Existing rules of the old kind will be ignored. To switch your codebase from a builtin     |
| kind to a mapped kind, use `buildozer`_.                                                   |
+------------------------------------------------------------+-------------------------------+
| :direc:`# gazelle:alias_kind macro_name wrapped_kind`      | n/a                           |
+------------------------------------------------------------+-------------------------------+
| Denotes that a macro aliases / wraps a given rule.                                         |
|                                                                                            |
| If you have a wrapper macro around a rule that gazelle knows how to update the attrs for,  |
| the alias_kind directive will instruct gazelle that it should treat the particular marco   |
| like the underlying wrapped kind.                                                          |
|                                                                                            |
| ``alias_kind`` is different from the ``map_kind`` directive in that it does not force the  |
| rule to be generated as the wrapped kind. Instead, it just instructs gazelle that it       |
| should index and update the attrs for rules that match the macro.                          |
|                                                                                            |
| For example, if you use ``# gazelle:alias_kind my_foo_binary foo_binary``, Gazelle will    |
| still generate ``foo_binary`` targets when generating new targets from new source files.   |
| It is up to a person to update the ``foo_binary`` targets to ``my_foo_binary`` targets.    |
| Once this manual step is done, Gazelle will continue to update the ``my_foo_binary``       |
| targets as if they were ``foo_binary`` targets.                                            |
|                                                                                            |
| Wrapper macros are commonly used to handle common boilerplate or to add deploy/release     |
| verbs, as described in the bazel `Verbs Tutorial`_.                                        |
|                                                                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:prefix path`                    | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| A prefix for ``importpath`` attributes on library rules. Gazelle will set                  |
| an ``importpath`` on a ``go_library`` or ``go_proto_library`` by                           |
| concatenating this with the relative path from the directory where the                     |
| prefix is set to the library. Most commonly, ``prefix`` is set to the                      |
| name of a repository in the root directory of a repository. For example,                   |
| in this repository, ``prefix`` is set in ``//:BUILD.bazel`` to                             |
| ``github.com/bazelbuild/bazel-gazelle``. The ``go_library`` in                             |
| ``//cmd/gazelle`` is assigned the ``importpath``                                           |
| ``"github.com/bazelbuild/bazel-gazelle/cmd/gazelle"``.                                     |
|                                                                                            |
| As a special case, when Gazelle enters a directory named ``vendor``, it sets               |
| ``prefix`` to the empty string. This automatically gives vendored libraries                |
| an intuitive ``importpath``.                                                               |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:proto mode`                     | :value:`default`                       |
+---------------------------------------------------+----------------------------------------+
| Tells Gazelle how to generate rules for .proto files. Valid values are:                    |
|                                                                                            |
| * ``default``: ``proto_library``, ``go_proto_library``, and ``go_library``                 |
|   rules are generated using ``@io_bazel_rules_go//proto:def.bzl``. Only one                |
|   of each rule may be generated per directory. This is the default mode.                   |
| * ``file``: a ``proto_library`` rule is generated for every .proto file.                   |
| * ``package``: multiple ``proto_library`` and ``go_proto_library`` rules                   |
|   may be generated in the same directory. .proto files are grouped into                    |
|   rules based on their package name or another option (see ``proto_group``).               |
| * ``legacy``: ``filegroup`` rules are generated for use by                                 |
|   ``@io_bazel_rules_go//proto:go_proto_library.bzl``. ``go_proto_library``                 |
|   rules must be written by hand. Gazelle will run in this mode automatically               |
|   if ``go_proto_library.bzl`` is loaded to avoid disrupting existing                       |
|   projects, but this can be overridden with a directive.                                   |
| * ``disable``: .proto files are ignored. Gazelle will run in this mode                     |
|   automatically if ``go_proto_library`` is loaded from any other source,                   |
|   but this can be overridden with a directive.                                             |
| * ``disable_global``: like ``disable`` mode, but also prevents Gazelle from                |
|   using any special cases in dependency resolution for Well Known Types and                |
|   Google APIs. Useful for avoiding build-time dependencies on protoc.                      |
|                                                                                            |
| This directive applies to the current directory and subdirectories. As a                   |
| special case, when Gazelle enters a directory named ``vendor``, if the proto               |
| mode isn't set explicitly in a parent directory or on the command line,                    |
| Gazelle will run in ``disable`` mode. Additionally, if the file                            |
| ``@io_bazel_rules_go//proto:go_proto_library.bzl`` is loaded, Gazelle                      |
| will run in ``legacy`` mode.                                                               |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:proto_group option`             | :value:`""`                            |
+---------------------------------------------------+----------------------------------------+
| *This directive is only effective in* ``package`` *mode (see above).*                      |
|                                                                                            |
| Specifies an option that Gazelle can use to group .proto files into rules.                 |
| For example, when set to ``go_package``, .proto files with the same                        |
| ``option go_package`` will be grouped together.                                            |
|                                                                                            |
| When this directive is set to the empty string, Gazelle will group packages                |
| by their proto package statement.                                                          |
|                                                                                            |
| Rule names are generated based on the last run of identifier characters                    |
| in the package name. For example, if the package is ``"foo/bar/baz"``, the                 |
| ``proto_library`` rule will be named ``baz_proto``.                                        |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:proto_import_prefix path`       | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Sets the `import_prefix`_ attribute of generated ``proto_library`` rules.                  |
| This adds a prefix to the string used to import ``.proto`` files listed in                 |
| the ``srcs`` attribute of generated rules.                                                 |
|                                                                                            |
| For example, if the target ``//a:b_proto`` has ``srcs = ["b.proto"]`` and                  |
| ``import_prefix = "github.com/x/y"``, then ``b.proto`` should be imported                  |
| with the string ``"github.com/x/y/a/b.proto"``.                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:proto_strip_import_prefix path` | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Sets the `strip_import_prefix`_ attribute of generated ``proto_library`` rules.            |
| This is a prefix to strip from the strings used to import ``.proto`` files.                |
|                                                                                            |
| If the prefix starts with a slash, it's intepreted relative to the repository              |
| root. Otherwise, it's relative to the directory containing the build file.                 |
| The package-relative form is only useful when a single build file covers                   |
| ``.proto`` files in subdirectories. Gazelle doesn't generate build files like              |
| this, so only paths with a leading slash should be used. Gazelle will print                |
| a warning when the package-relative form is used.                                          |
|                                                                                            |
| For example, if the target ``//proto/a:b_proto`` has ``srcs = ["b.proto"]``                |
| and ``strip_import_prefix = "/proto"``, then ``b.proto`` should be imported                |
| with the string ``"a/b.proto"``.                                                           |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:resolve ...`                    | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Specifies an explicit mapping from an import string to a label for                         |
| `Dependency resolution`_. The format for a resolve directive is:                           |
|                                                                                            |
| ``# gazelle:resolve source-lang import-lang import-string label``                          |
|                                                                                            |
| * ``source-lang`` is the language of the source code being imported.                       |
| * ``import-lang`` is the language importing the library. This is usually                   |
|   the same as ``source-lang`` but may differ with generated code. For                      |
|   example, when resolving dependencies for a ``go_proto_library``,                         |
|   ``source-lang`` would be ``"proto"`` and ``import-lang`` would be ``"go"``.              |
|   ``import-lang`` may be omitted if it is the same as ``source-lang``.                     |
| * ``import-string`` is the string used in source code to import a library.                 |
| * ``label`` is the Bazel label that Gazelle should write in ``deps``.                      |
|                                                                                            |
| For example:                                                                               |
|                                                                                            |
| .. code:: bzl                                                                              |
|                                                                                            |
|   # gazelle:resolve go example.com/foo //foo:go_default_library                            |
|   # gazelle:resolve proto go foo/foo.proto //foo:foo_go_proto                              |
|                                                                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:resolve_regexp ...`             | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Specifies an explicit mapping from an import regex to a label for                          |
| `Dependency resolution`_. The format for a resolve directive is:                           |
|                                                                                            |
| ``# gazelle:resolve_regexp source-lang import-lang import-string-regex label``             |
|                                                                                            |
| * ``source-lang`` is the language of the source code being imported.                       |
| * ``import-lang`` is the language importing the library. This is usually                   |
|   the same as ``source-lang`` but may differ with generated code. For                      |
|   example, when resolving dependencies for a ``go_proto_library``,                         |
|   ``source-lang`` would be ``"proto"`` and ``import-lang`` would be ``"go"``.              |
|   ``import-lang`` may be omitted if it is the same as ``source-lang``.                     |
| * ``import-string-regex`` is the regex applied to the import in the source code.           |
|   If it matches, that import will be resolved to the label specified below.                |
| * ``label`` is the Bazel label that Gazelle should write in ``deps``. The label            |
|   can be constructed using captured strings from the subpattern matching in                |
|   import-string-regex                                                                      |
|                                                                                            |
| For example:                                                                               |
|                                                                                            |
| .. code:: bzl                                                                              |
|                                                                                            |
|   # gazelle:resolve_regexp go example.com/.* //foo:go_default_library                      |
|   # gazelle:resolve_regexp proto go foo/.*\.proto //foo:foo_go_proto                       |
|   # gazelle:resolve_regexp proto go foo/(.*)\.proto //foo/$1:foo_rule_proto                |
|                                                                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:go_visibility label`            | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| By default, internal packages are only visible to its siblings. This directive adds a label|
| internal packages should be visible to additionally. This directive can be used several    |
| times, adding a list of labels.                                                            |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:lang lang1,lang2,...`           | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Sets the language selection flag for this and descendent packages, which causes gazelle to |
| index and generate rules for only the languages named in this directive.                   |
+---------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:default_visibility visibility`  | n/a                                    |
+---------------------------------------------------+----------------------------------------+
| Comma-separated list of visibility specifications.                                         |
| This directive adds the visibility specifications for this and descendant packages.        |
|                                                                                            |
| For example:                                                                               |
|                                                                                            |
| .. code:: bzl                                                                              |
|                                                                                            |
|   # gazelle:default_visibility //foo:__subpackages__,//src:__subpackages__                 |
+---------------------------------------------------+----------------------------------------+

Gazelle also reads directives from the WORKSPACE file. They may be used to
discover custom repository names and known prefixes. The ``fix`` and ``update``
commands use these directives for dependency resolution. ``update-repos`` uses
them to learn about repository rules defined in alternate locations.

+--------------------------------------------------------------------+----------------------------------------+
| **WORKSPACE Directive**                                            | **Default value**                      |
+====================================================================+========================================+
| :direc:`# gazelle:repository_macro [+]macroFile%defName`           | n/a                                    |
+--------------------------------------------------------------------+----------------------------------------+
| Tells Gazelle to look for repository rules in a macro in a .bzl file. The directive can be                  |
| repeated multiple times.                                                                                    |
| The macro can be generated by calling ``update-repos`` with the ``to_macro`` flag.                          |
|                                                                                                             |
| The directive can be prepended with a "+", which will tell Gazelle to also look for repositories            |
| within any macros called by the specified macro.                                                            |
+--------------------------------------------------------------------+----------------------------------------+
| :direc:`# gazelle:repository rule_kind attr1_name=attr1_value ...` | n/a                                    |
+--------------------------------------------------------------------+----------------------------------------+
| Specifies a repository rule that Gazelle should know about. The directive can be repeated multiple times,   |
| and can be declared from within a macro definition that Gazelle knows about. At the very least the          |
| directive must define a rule kind and a name attribute, but it can define extra attributes after that.      |
|                                                                                                             |
| This is useful for teaching Gazelle about repos declared in external macros. The directive can also be used |
| to override an actual repository rule. For example, a ``git_repository`` rule for ``org_golang_x_tools``    |
| could be overriden with the directive:                                                                      |
|                                                                                                             |
| .. code:: bzl                                                                                               |
|                                                                                                             |
|   # gazelle:repository go_repository name=org_golang_x_tools importpath=golang.org/x/tools                  |
|                                                                                                             |
| Gazelle would then proceed as if ``org_golang_x_tools`` was declared as a ``go_repository`` rule.           |
+--------------------------------------------------------------------+----------------------------------------+

Keep comments
~~~~~~~~~~~~~

In addition to directives, Gazelle supports ``# keep`` comments that protect
parts of build files from being modified. ``# keep`` may be written before
a rule, before an attribute, or after a string within a list.

``# keep`` comments might take one of 2 forms; the ``# keep`` literal or a
description prefixed by ``# keep:``.

Example
^^^^^^^

Suppose you have a library that includes a generated .go file. Gazelle won't
know what imports to resolve, so you may need to add dependencies manually with
``# keep`` comments.

.. code:: bzl

  load("@io_bazel_rules_go//go:def.bzl", "go_library")
  load("@com_github_example_gen//:gen.bzl", "gen_go_file")

  gen_go_file(
      name = "magic",
      srcs = ["magic.go.in"],
      outs = ["magic.go"],
  )

  go_library(
      name = "go_default_library",
      srcs = ["magic.go"],
      visibility = ["//visibility:public"],
      deps = [
          "@com_github_example_gen//:go_default_library",  # keep
          "@com_github_example_gen//a/b/c:go_default_library",  # keep: this is also important
      ],
  )

Dependency resolution
---------------------

One of Gazelle's most important jobs is resolving library import strings
(like ``import "golang.org/x/sys/unix"``) to Bazel labels (like
``@org_golang_x_sys//unix:go_default_library``). Gazelle follows the rules
below to resolve dependencies:

1. If the import to be resolved is part of a standard library, no explicit
   dependency is written. For example, in Go, you don't need to declare
   that you depend on ``"fmt"``.
2. If a ``# gazelle:resolve`` directive matches the import to be resolved,
   the label at the end of the directive will be used.
3. If proto rule generation is enabled, special rules will be used when
   importing certain libraries. These rules may be disabled by adding
   ``# gazelle:proto disable_global`` to a build file (this will affect
   subdirectories, too) or by passing ``-proto disable_global`` on the
   command line.

   a) Imports of Well Known Types are mapped to rules in
      ``@io_bazel_rules_go//proto/wkt``.
   b) Imports of Google APIs are mapped to ``@go_googleapis``.
   c) Imports of ``github.com/golang/protobuf/ptypes``, ``descriptor``, and
      ``jsonpb`` are mapped to special rules in ``@com_github_golang_protobuf``.
      See `Avoiding conflicts with proto rules`_.

4. If the import to be resolved is in the library index, the import will be resolved
   to that library. If ``-index=true``, Gazelle builds an index of library rules in
   the current repository before starting dependency resolution, and this is how
   most dependencies are resolved.

   a) For Go, the match is based on the ``importpath`` attribute.
   b) For proto, the match is based on the ``srcs`` attribute.

5. If ``-index=false`` and a package is imported that has the current ``go_prefix``
   as a prefix, Gazelle generates a label following a convention. For example, if
   the build file in ``//src`` set the prefix with
   ``# gazelle:prefix example.com/repo/foo``, and you import the library
   ``"example.com/repo/foo/bar``, the dependency will be
   ``"//src/foo/bar:go_default_library"``.
6. Otherwise, Gazelle will use the current ``external`` mode to resolve
   the dependency.

   a) In ``external`` mode (the default), Gazelle will transform the import
      string into an external repository label. For example,
      ``"golang.org/x/sys/unix"`` would be resolved to
      ``"@org_golang_x_sys//unix:go_default_library"``. Gazelle does not confirm
      whether the external repository is actually declared in WORKSPACE,
      but if there *is* a ``go_repository`` in WORKSPACE with a matching
      ``importpath``, Gazelle will use its name. Gazelle does not index
      rules in external repositories, so it's possible the resolved dependency
      does not exist.
   b) In ``static`` mode, Gazelle has the same behavior as ``external`` mode,
      except that it will not call out to the network for resolution when no
      matching import is found within WORKSPACE. Instead, it will skip the
      unknown import. This is the default mode for ``go_repository`` rules.
   c) In ``vendored`` mode, Gazelle will transform the import string into
      a label in the vendor directory. For example, ``"golang.org/x/sys/unix"``
      would be resolved to
      ``"//vendor/golang.org/x/sys/unix:go_default_library"``. This mode is
      usually not necessary, since vendored libraries will be indexed and
      resolved using rule 4.

Fix command transformations
---------------------------

Gazelle will generate and update build files when invoked with either
``gazelle update`` or ``gazelle fix`` (``update`` is the default). Both commands
perform several transformations to fix deprecated usage of the Go rules.
``update`` performs a safe set of tranformations, while ``fix`` performs some
additional transformations that may delete or rename rules.

The following transformations are performed:

**Migrate library to embed (fix and update):** Gazelle replaces ``library``
attributes with ``embed`` attributes.

**Migrate gRPC compilers (fix and update):** Gazelle converts
``go_grpc_library`` rules to ``go_proto_library`` rules with
``compilers = ["@io_bazel_rules_go//proto:go_grpc"]``.

**Flatten srcs (fix and update):** Gazelle converts ``srcs`` attributes that
use OS and architecture-specific ``select`` expressions to flat lists.
rules_go filters these sources anyway.

**Squash cgo libraries (fix only)**: Gazelle will remove `cgo_library` rules
named ``cgo_default_library`` and merge their attributes with a ``go_library``
rule in the same package named ``go_default_library``. If no such ``go_library``
rule exists, a new one will be created. Other ``cgo_library`` rules will not be
removed.

**Squash external tests (fix only)**: Gazelle will squash ``go_test`` rules
named ``go_default_xtest`` into ``go_default_test``. Earlier versions of
rules_go required internal and external tests to be built separately, but
this is no longer needed.

**Remove legacy protos (fix only)**: Gazelle will remove usage of
``go_proto_library`` rules loaded from
``@io_bazel_rules_go//proto:go_proto_library.bzl`` and ``filegroup`` rules named
``go_default_library_protos``. Newly generated proto rules will take their
place. Since ``filegroup`` isn't needed anymore and ``go_proto_library`` has
different attributes and was always written by hand, Gazelle will not attempt to
merge anything from these rules with the newly generated rules.

This transformation is only applied in the default proto mode. Since Gazelle
will run in legacy proto mode if ``go_proto_library.bzl`` is loaded, this
transformation is not usually applied. You can set the proto mode explicitly
using the directive ``# gazelle:proto default``.

**Update loads of gazelle rule (fix and update)**: Gazelle will remove loads
of ``gazelle`` from ``@io_bazel_rules_go//go:def.bzl``. It will automatically
add a load from ``@bazel_gazelle//:def.bzl`` if ``gazelle`` is not loaded
from another location.
