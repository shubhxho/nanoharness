// Package version reports nanoharness build identity from ldflags and VCS metadata.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Injected by -ldflags at build/install time (see Makefile).
var (
	Version = "dev"
	Commit  = ""
)

// Resolve returns the best available version string (tag or module version).
func Resolve() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

// Rev returns the short git commit from ldflags or embedded VCS build info.
func Rev() string {
	if Commit != "" {
		return short(Commit)
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev string
		modified := false
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
		if rev != "" {
			out := short(rev)
			if modified {
				out += "-dirty"
			}
			return out
		}
	}
	return ""
}

// Full is "version (rev)" when a revision is known.
func Full() string {
	v := Resolve()
	if rev := Rev(); rev != "" {
		return fmt.Sprintf("%s (%s)", v, rev)
	}
	return v
}

func short(rev string) string {
	rev = strings.TrimSpace(rev)
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}
