workspace(name = "bazel_gazelle")

load(
    "@bazel_tools//tools/build_defs/repo:git.bzl",
    "git_repository",
)

git_repository(
    name = "bazel_skylib",
    commit = "df3c9e2735f02a7fe8cd80db4db00fec8e13d25f",  # `master` as of 2021-08-19
    remote = "https://github.com/bazelbuild/bazel-skylib",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f4a9314518ca6acfa16cc4ab43b0b8ce1e4ea64b81c38d8a3772883f153346b8",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.50.1/rules_go-v0.50.1.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.50.1/rules_go-v0.50.1.zip",
    ],
)

# TODO: The rules_go bazel_features shim doesn't provide targets for .bzl files.
http_archive(
    name = "bazel_features",
    sha256 = "d7787da289a7fb497352211ad200ec9f698822a9e0757a4976fd9f713ff372b3",
    strip_prefix = "bazel_features-1.9.1",
    url = "https://github.com/bazel-contrib/bazel_features/releases/download/v1.9.1/bazel_features-v1.9.1.tar.gz",
)

load("@bazel_features//:deps.bzl", "bazel_features_deps")

bazel_features_deps()

load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_register_nogo", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.23.3")

go_register_nogo(
    excludes = [
        # //cmd/gazelle:gazelle_lib has incomplete deps on purpose, which
        # trips up nogo.
        "@//cmd/gazelle:__pkg__",
    ],
    nogo = "@bazel_gazelle//:nogo",
)

# Go 1.22 is needed since the non-hermeticity of GoToolchainBinaryBuild results in it downloading
# Go 1.23 on Windows to build the builder, which then messes up Go version build tag filtering.
# Go 1.21 is needed to support the toolchain directive in go.mod, which is non-hermetically read
# by GoToolchainBinaryBuild on Windows.
# Go 1.20 is needed so support nogo's use of token.File.FileStart.
# Go 1.19 is needed for recent versions of golang.org/x/tools.
# TODO: Fix rules_go and set this back to 1.19.
go_download_sdk(
    name = "go_compat_sdk",
    version = "1.22.9",
)

load("//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies(go_sdk = "go_sdk")

# Needed by rules_go when using proto rules, but not provided by its deps macro.
http_archive(
    name = "rules_proto",
    sha256 = "6fb6767d1bef535310547e03247f7518b03487740c11b6c6adb7952033fe1295",
    strip_prefix = "rules_proto-6.0.2",
    url = "https://github.com/bazelbuild/rules_proto/releases/download/6.0.2/rules_proto-6.0.2.tar.gz",
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")

rules_proto_dependencies()

load("@rules_proto//proto:setup.bzl", "rules_proto_setup")

rules_proto_setup()

load("@rules_proto//proto:toolchains.bzl", "rules_proto_toolchains")

rules_proto_toolchains()

# For API doc generation
# This is a dev dependency, users should not need to install it
# so we declare it in the WORKSPACE
http_archive(
    name = "io_bazel_stardoc",
    sha256 = "62bd2e60216b7a6fec3ac79341aa201e0956477e7c8f6ccc286f279ad1d96432",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/stardoc/releases/download/0.6.2/stardoc-0.6.2.tar.gz",
        "https://github.com/bazelbuild/stardoc/releases/download/0.6.2/stardoc-0.6.2.tar.gz",
    ],
)

# Stardoc pulls in a lot of deps, which we need to declare here.
load("@io_bazel_stardoc//:setup.bzl", "stardoc_repositories")

stardoc_repositories()

load("@rules_jvm_external//:repositories.bzl", "rules_jvm_external_deps")

rules_jvm_external_deps()

load("@rules_jvm_external//:setup.bzl", "rules_jvm_external_setup")

rules_jvm_external_setup()

load("@io_bazel_stardoc//:deps.bzl", "stardoc_external_deps")

stardoc_external_deps()

load("@stardoc_maven//:defs.bzl", stardoc_pinned_maven_install = "pinned_maven_install")

stardoc_pinned_maven_install()

load("@bazel_skylib//lib:unittest.bzl", "register_unittest_toolchains")

register_unittest_toolchains()

# gazelle:repository go_repository name=cat_dario_mergo importpath=dario.cat/mergo
# gazelle:repository go_repository name=com_github_anmitsu_go_shlex importpath=github.com/anmitsu/go-shlex
# gazelle:repository go_repository name=com_github_armon_go_socks5 importpath=github.com/armon/go-socks5
# gazelle:repository go_repository name=com_github_bazelbuild_buildtools importpath=github.com/bazelbuild/buildtools
# gazelle:repository go_repository name=com_github_bazelbuild_rules_go importpath=github.com/bazelbuild/rules_go
# gazelle:repository go_repository name=com_github_bmatcuk_doublestar_v4 importpath=github.com/bmatcuk/doublestar/v4
# gazelle:repository go_repository name=com_github_cloudflare_circl importpath=github.com/cloudflare/circl
# gazelle:repository go_repository name=com_github_cyphar_filepath_securejoin importpath=github.com/cyphar/filepath-securejoin
# gazelle:repository go_repository name=com_github_davecgh_go_spew importpath=github.com/davecgh/go-spew
# gazelle:repository go_repository name=com_github_elazarl_goproxy importpath=github.com/elazarl/goproxy
# gazelle:repository go_repository name=com_github_emirpasic_gods importpath=github.com/emirpasic/gods
# gazelle:repository go_repository name=com_github_fsnotify_fsnotify importpath=github.com/fsnotify/fsnotify
# gazelle:repository go_repository name=com_github_gliderlabs_ssh importpath=github.com/gliderlabs/ssh
# gazelle:repository go_repository name=com_github_go_git_gcfg importpath=github.com/go-git/gcfg
# gazelle:repository go_repository name=com_github_go_git_go_billy_v5 importpath=github.com/go-git/go-billy/v5
# gazelle:repository go_repository name=com_github_go_git_go_git_fixtures_v4 importpath=github.com/go-git/go-git-fixtures/v4
# gazelle:repository go_repository name=com_github_go_git_go_git_v5 importpath=github.com/go-git/go-git/v5
# gazelle:repository go_repository name=com_github_gogo_protobuf importpath=github.com/gogo/protobuf
# gazelle:repository go_repository name=com_github_golang_groupcache importpath=github.com/golang/groupcache
# gazelle:repository go_repository name=com_github_golang_mock importpath=github.com/golang/mock
# gazelle:repository go_repository name=com_github_golang_protobuf importpath=github.com/golang/protobuf
# gazelle:repository go_repository name=com_github_google_go_cmp importpath=github.com/google/go-cmp
# gazelle:repository go_repository name=com_github_jbenet_go_context importpath=github.com/jbenet/go-context
# gazelle:repository go_repository name=com_github_kevinburke_ssh_config importpath=github.com/kevinburke/ssh_config
# gazelle:repository go_repository name=com_github_kr_pretty importpath=github.com/kr/pretty
# gazelle:repository go_repository name=com_github_kr_text importpath=github.com/kr/text
# gazelle:repository go_repository name=com_github_microsoft_go_winio importpath=github.com/Microsoft/go-winio
# gazelle:repository go_repository name=com_github_onsi_gomega importpath=github.com/onsi/gomega
# gazelle:repository go_repository name=com_github_pjbgf_sha1cd importpath=github.com/pjbgf/sha1cd
# gazelle:repository go_repository name=com_github_pkg_errors importpath=github.com/pkg/errors
# gazelle:repository go_repository name=com_github_pmezard_go_difflib importpath=github.com/pmezard/go-difflib
# gazelle:repository go_repository name=com_github_protonmail_go_crypto importpath=github.com/ProtonMail/go-crypto
# gazelle:repository go_repository name=com_github_rogpeppe_go_internal importpath=github.com/rogpeppe/go-internal
# gazelle:repository go_repository name=com_github_sergi_go_diff importpath=github.com/sergi/go-diff
# gazelle:repository go_repository name=com_github_skeema_knownhosts importpath=github.com/skeema/knownhosts
# gazelle:repository go_repository name=com_github_stretchr_testify importpath=github.com/stretchr/testify
# gazelle:repository go_repository name=com_github_xanzy_ssh_agent importpath=github.com/xanzy/ssh-agent
# gazelle:repository go_repository name=in_gopkg_check_v1 importpath=gopkg.in/check.v1
# gazelle:repository go_repository name=in_gopkg_warnings_v0 importpath=gopkg.in/warnings.v0
# gazelle:repository go_repository name=in_gopkg_yaml_v3 importpath=gopkg.in/yaml.v3
# gazelle:repository go_repository name=net_starlark_go importpath=go.starlark.net
# gazelle:repository go_repository name=org_golang_google_genproto importpath=google.golang.org/genproto
# gazelle:repository go_repository name=org_golang_google_grpc importpath=google.golang.org/grpc
# gazelle:repository go_repository name=org_golang_google_grpc_cmd_protoc_gen_go_grpc importpath=google.golang.org/grpc/cmd/protoc-gen-go-grpc
# gazelle:repository go_repository name=org_golang_google_protobuf importpath=google.golang.org/protobuf
# gazelle:repository go_repository name=org_golang_x_crypto importpath=golang.org/x/crypto
# gazelle:repository go_repository name=org_golang_x_mod importpath=golang.org/x/mod
# gazelle:repository go_repository name=org_golang_x_net importpath=golang.org/x/net
# gazelle:repository go_repository name=org_golang_x_sync importpath=golang.org/x/sync
# gazelle:repository go_repository name=org_golang_x_sys importpath=golang.org/x/sys
# gazelle:repository go_repository name=org_golang_x_term importpath=golang.org/x/term
# gazelle:repository go_repository name=org_golang_x_text importpath=golang.org/x/text
# gazelle:repository go_repository name=org_golang_x_tools importpath=golang.org/x/tools
# gazelle:repository go_repository name=org_golang_x_tools_go_vcs importpath=golang.org/x/tools/go/vcs
