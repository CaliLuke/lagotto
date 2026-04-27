package main

import (
	"fmt"
	"runtime/debug"
)

// version is overridden via -ldflags at release time by goreleaser
// (e.g. "v0.1.0"). When unset (a `go install` build, a `go build`
// from a dirty checkout), it falls back to module info from
// runtime/debug.ReadBuildInfo and finally to "(devel)".
var version = ""

// versionString returns the resolved tool version, including VCS
// metadata (commit hash + dirty flag) when the binary was built
// without explicit -ldflags. Used by the `--version` flag.
func versionString() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		v = "(devel)"
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				rev = s.Value[:12]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	if rev == "" {
		return v
	}
	return fmt.Sprintf("%s (%s%s)", v, rev, dirty)
}
