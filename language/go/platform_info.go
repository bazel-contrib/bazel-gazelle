//go:generate bazel run --run_under=cp //language/go:gen_platform_info.go ./platform_info.go

package golang

// These helpers are faster than map lookups using 'rule.KnownOSSet'/'rule.KnownArchSet'.

func IsKnownOS(os string) bool {
	switch os {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "osx", "plan9", "qnx", "solaris", "windows":
		return true
	default:
		return false
	}
}

func IsKnownArch(arch string) bool {
	switch arch {
	case "386", "amd64", "arm", "arm64", "mips", "mips64", "mips64le", "mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "wasm":
		return true
	default:
		return false
	}
}
