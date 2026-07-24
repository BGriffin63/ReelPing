// Package version exposes build metadata that is injected at build time via
// -ldflags. It has no dependencies so every other package can import it freely.
package version

import "runtime"

// These variables are overridden at build time, e.g.:
//
//	go build -ldflags "-X github.com/BGriffin63/reelping/internal/version.Version=1.0.0 ..."
var (
	// Version is the semantic version of this build.
	Version = "0.5.0-beta"
	// Commit is the short git commit the build was produced from.
	Commit = "unknown"
	// Date is the build date (RFC3339 or "unknown").
	Date = "unknown"
)

// Info is a serialisable snapshot of build metadata.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current build info.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a compact human-readable version string.
func String() string {
	return "ReelPing " + Version + " (" + Commit + ")"
}
