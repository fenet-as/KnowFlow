package platform

var (
	// Version is the application version injected at compile time via -ldflags.
	Version = "dev"
	// Commit is the git commit SHA injected at compile time via -ldflags.
	Commit = "none"
	// BuildTime is the UTC timestamp injected at compile time via -ldflags.
	BuildTime = "unknown"
)

// VersionInfo models the payload returned by the /version endpoint.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// GetVersionInfo returns the current build information.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}

