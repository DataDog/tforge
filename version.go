// Package tforge is the root of the github.com/DataDog/tforge module. It
// exists solely to hold the single Version variable shared between the
// `tforge` CLI's version subcommand and the `render` package's JSON metadata
// stamp — see the doc on Version below.
package tforge

import (
	"runtime/debug"
)

// Version is the canonical tforge version. It is the single source of truth
// used by:
//
//   - the `tforge version` CLI subcommand, and
//   - render.ToJSON / render.ToFile, which embed it in the
//     `//.metadata.version` field of generated Terraform JSON.
//
// Resolution order, evaluated once at package init:
//
//  1. The link-time override (the release workflow injects
//     `-ldflags '-X github.com/DataDog/tforge.injectedVersion=vX.Y.Z'`).
//  2. `Main.Version` from `runtime/debug.ReadBuildInfo`, which `go install`
//     populates for module-cache builds such as
//     `go install github.com/DataDog/tforge/cmd/tforge@v0.3.0`.
//  3. `"dev+<7-char vcs.revision>"` (suffixed `+dirty` when the working tree
//     differs from HEAD), for in-tree builds such as
//     `go install ./cmd/tforge` from a checkout.
//  4. `"dev"` as a last-resort sentinel.
//
// This means: release artifacts carry the tag; `go install ...@tag` carries
// the tag for free without ldflags; source builds carry the commit hash so
// reported versions are at least traceable.
var Version = resolveVersion(injectedVersion, debug.ReadBuildInfo)

// injectedVersion is the link-time override target. Release builds set this
// via `-X github.com/DataDog/tforge.injectedVersion=<tag>`. Normal builds
// leave it empty.
var injectedVersion string

// readBuildInfoFunc matches the signature of runtime/debug.ReadBuildInfo, so
// tests can substitute a stub.
type readBuildInfoFunc func() (*debug.BuildInfo, bool)

// resolveVersion implements the four-step resolution documented on Version.
// Split out from the package-level initializer so it is testable without
// shelling out to real binaries.
func resolveVersion(injected string, readBuildInfo readBuildInfoFunc) string {
	if injected != "" {
		return injected
	}
	info, ok := readBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if len(revision) >= 7 {
		short := "dev+" + revision[:7]
		if modified == "true" {
			short += "+dirty"
		}
		return short
	}
	return "dev"
}
