// Package version exposes the build version. It is populated by the
// linker (-X version.Version=…) during release builds; in dev it
// stays "dev".
package version

// Version is overwritten via ldflags at build time, e.g.:
//
//	go build -ldflags "-X predictdestiny/internal/version.Version=$(git describe --tags)" ./cmd/server
var Version = "dev"
