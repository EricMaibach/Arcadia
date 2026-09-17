package ai

import (
	"fmt"
	"runtime"
	"time"
)

// Version information for the AI module
const (
	// Major version number
	VersionMajor = 1

	// Minor version number
	VersionMinor = 0

	// Patch version number
	VersionPatch = 0

	// Pre-release identifier (empty for stable releases)
	VersionPrerelease = ""

	// Build metadata (set during build process)
	VersionBuild = "dev"
)

// Version returns the complete version string
func Version() string {
	version := fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)

	if VersionPrerelease != "" {
		version = fmt.Sprintf("%s-%s", version, VersionPrerelease)
	}

	if VersionBuild != "" && VersionBuild != "dev" {
		version = fmt.Sprintf("%s+%s", version, VersionBuild)
	}

	return version
}

// VersionInfo contains detailed version information
type VersionInfo struct {
	Version      string `json:"version"`
	Major        int    `json:"major"`
	Minor        int    `json:"minor"`
	Patch        int    `json:"patch"`
	Prerelease   string `json:"prerelease,omitempty"`
	Build        string `json:"build,omitempty"`
	BuildTime    string `json:"build_time"`
	GitCommit    string `json:"git_commit"`
	GitBranch    string `json:"git_branch"`
	GoVersion    string `json:"go_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
}

// Build-time variables (set by build system)
var (
	BuildTime = "unknown"
	GitCommit = "unknown"
	GitBranch = "unknown"
)

// GetVersionInfo returns detailed version information
func GetVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version:      Version(),
		Major:        VersionMajor,
		Minor:        VersionMinor,
		Patch:        VersionPatch,
		Prerelease:   VersionPrerelease,
		Build:        VersionBuild,
		BuildTime:    BuildTime,
		GitCommit:    GitCommit,
		GitBranch:    GitBranch,
		GoVersion:    runtime.Version(),
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// UserAgent returns a user agent string for HTTP requests
func UserAgent() string {
	return fmt.Sprintf("ArcadiaAI/%s (%s; %s)", Version(), runtime.GOOS, runtime.GOARCH)
}

// IsStableVersion returns true if this is a stable release (no prerelease)
func IsStableVersion() bool {
	return VersionPrerelease == ""
}

// IsDevelopmentBuild returns true if this is a development build
func IsDevelopmentBuild() bool {
	return VersionBuild == "dev" || BuildTime == "unknown" || GitCommit == "unknown"
}

// CompatibilityVersion returns the compatibility version for API versioning
func CompatibilityVersion() string {
	return fmt.Sprintf("v%d", VersionMajor)
}

// ModuleInfo returns module information for registration
func GetModuleInfo() *ModuleInfo {
	now := time.Now()
	buildTime := BuildTime
	if buildTime == "unknown" {
		buildTime = now.Format(time.RFC3339)
	}

	var uptime string
	if startTime, err := time.Parse(time.RFC3339, buildTime); err == nil {
		uptime = now.Sub(startTime).String()
	}

	features := []string{
		"messaging",
		"context_management",
		"tool_execution",
		"provider_management",
	}

	// Add conditional features based on build
	if !IsDevelopmentBuild() {
		features = append(features, "metrics", "health_checks")
	}

	return &ModuleInfo{
		Version:        Version(),
		BuildTime:      buildTime,
		GitCommit:      GitCommit,
		Features:       features,
		Config:         make(map[string]interface{}),
		Status:         "initialized",
		StartedAt:      now.Format(time.RFC3339),
		Uptime:         uptime,
		Providers:      []string{"openai"}, // Default supported providers
		ActiveProvider: "openai",           // Default active provider
	}
}

// Constants for module identification
const (
	ModuleType = "ai"
	// ModuleName is defined in facade.go to avoid duplication
	ModuleDescription = "AI services module for Arcadia platform"
	ModuleAuthor      = "Arcadia Team"
	ModuleLicense     = "Proprietary"
	ModuleURL         = "https://github.com/arcadia/appengine"
)

// API version constants
const (
	APIVersionV1 = "v1"
	APIVersionV2 = "v2" // For future use
)

// Supported API versions
var SupportedAPIVersions = []string{
	APIVersionV1,
}

// GetSupportedAPIVersions returns a list of supported API versions
func GetSupportedAPIVersions() []string {
	return SupportedAPIVersions
}

// IsAPIVersionSupported checks if an API version is supported
func IsAPIVersionSupported(version string) bool {
	for _, supported := range SupportedAPIVersions {
		if supported == version {
			return true
		}
	}
	return false
}

// GetLatestAPIVersion returns the latest supported API version
func GetLatestAPIVersion() string {
	if len(SupportedAPIVersions) > 0 {
		return SupportedAPIVersions[len(SupportedAPIVersions)-1]
	}
	return APIVersionV1
}
