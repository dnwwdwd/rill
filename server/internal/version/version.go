// Package version holds the application version injected at build time.
package version

// Version is overridden by the LPK build script from package.yml. The fallback
// keeps local `go run` and tests meaningful without extra flags.
var Version = "0.1.4"
