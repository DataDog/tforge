// Package tforge is the root of the github.com/DataDog/tforge module. It
// exists solely to hold the single Version variable shared between the
// `tforge` CLI's version subcommand and the `render` package's JSON metadata
// stamp — see the doc on Version below.
package tforge

// Version is the canonical tforge module version. It is the single source of
// truth used by:
//
//   - the `tforge version` CLI subcommand, and
//   - render.ToJSON / render.ToFile, which embed it in the `//.metadata.version`
//     field of generated Terraform JSON.
//
// The default value `"dev"` is overridden at link time by callers that know
// the real version — typically the version recorded in their go.mod entry
// for tforge:
//
//	go build -ldflags '-X github.com/DataDog/tforge.Version=v0.3.1-…'
//
// Keeping a single overridable variable (rather than reading runtime/debug
// build info, parsing go.mod, etc.) makes the value entirely owned by the
// consumer's build system. Whatever pins tforge stamps the value here, and
// the two consumer sites read it directly with no detection layer in between.
var Version = "dev"
