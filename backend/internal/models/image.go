package models

import "time"

// Status represents the comparison result for a container image.
type Status string

const (
	StatusUpToDate        Status = "up_to_date"
	StatusUpdateAvailable Status = "update_available"
	StatusUnknown         Status = "unknown"
	StatusCheckFailed     Status = "check_failed"
)

// ImageRecord is the canonical data model for a discovered container image.
type ImageRecord struct {
	// ID is a stable composite key: namespace:workloadKind:workloadName:containerName
	ID string `json:"id"`

	Namespace     string          `json:"namespace"`
	WorkloadKind  string          `json:"workloadKind"`
	WorkloadName  string          `json:"workloadName"`
	ContainerName string          `json:"containerName"`
	Management    *ManagementInfo `json:"management,omitempty"`

	// ConfiguredImage is the full image reference from the pod spec.
	ConfiguredImage string `json:"configuredImage"`
	Registry        string `json:"registry"`
	Repository      string `json:"repository"`
	Tag             string `json:"tag"`

	// RunningDigest is the digest of the image currently running in pods.
	// For multi-arch images this is the platform-specific (linux/amd64) digest.
	RunningDigest string `json:"runningDigest"`
	// RegistryDigest is the platform-specific digest the configured tag resolves
	// to in the registry. For multi-arch images this is the linux/amd64 digest —
	// the same format as RunningDigest — so they can be compared directly.
	RegistryDigest string `json:"registryDigest"`
	// IndexDigest is the manifest list digest for multi-arch images (shown on
	// Docker Hub as "Index Digest"). Empty for single-arch images.
	IndexDigest string `json:"indexDigest,omitempty"`

	// LatestTag is the newest compatible semver tag found in the registry for
	// this image's repository (e.g. "2.5.2" when the workload is running
	// "2.5.1"). Empty when unavailable or when the workload already uses the
	// latest compatible tag.
	LatestTag string `json:"latestTag,omitempty"`
	// LatestTagDigest is the digest the latest tag currently resolves to.
	LatestTagDigest string `json:"latestTagDigest,omitempty"`

	Platform string `json:"platform"`
	Status   Status `json:"status"`

	// PodNames lists names of pods currently using this image.
	PodNames []string `json:"podNames"`

	LastChecked *time.Time `json:"lastChecked"`
	Error       string     `json:"error,omitempty"`
}

// Summary is the payload for GET /api/v1/summary.
type Summary struct {
	TotalImages      int        `json:"totalImages"`
	UpToDate         int        `json:"upToDate"`
	UpdatesAvailable int        `json:"updatesAvailable"`
	Unknown          int        `json:"unknown"`
	CheckFailed      int        `json:"checkFailed"`
	UniqueRegistries int        `json:"uniqueRegistries"`
	LastScan         *time.Time `json:"lastScan"`
}

// ManagementInfo describes the tool that manages the owning workload, when it
// can be detected from Kubernetes metadata.
type ManagementInfo struct {
	Tool                 string `json:"tool"`
	ManagedBy            string `json:"managedBy,omitempty"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

// UpdateSummary is a concise update record for automation consumers.
type UpdateSummary struct {
	ID              string          `json:"id"`
	Image           string          `json:"image"`
	CurrentVersion  string          `json:"currentVersion"`
	LatestVersion   string          `json:"latestVersion,omitempty"`
	Namespace       string          `json:"namespace"`
	Workload        string          `json:"workload"`
	WorkloadKind    string          `json:"workloadKind"`
	WorkloadName    string          `json:"workloadName"`
	ContainerName   string          `json:"containerName"`
	Management      *ManagementInfo `json:"management,omitempty"`
	Registry        string          `json:"registry"`
	Repository      string          `json:"repository"`
	UpdateReason    string          `json:"updateReason"`
	LastChecked     *time.Time      `json:"lastChecked"`
	RunningDigest   string          `json:"runningDigest,omitempty"`
	RegistryDigest  string          `json:"registryDigest,omitempty"`
	LatestTagDigest string          `json:"latestTagDigest,omitempty"`
}

// UpdatesSummary is the payload for GET /api/v1/summary/updates.
type UpdatesSummary struct {
	Count    int             `json:"count"`
	LastScan *time.Time      `json:"lastScan"`
	Updates  []UpdateSummary `json:"updates"`
}

// RegistryInfo is the payload for GET /api/v1/registries.
type RegistryInfo struct {
	Hostname    string `json:"hostname"`
	ImageCount  int    `json:"imageCount"`
	Reachable   *bool  `json:"reachable"`
	AuthPresent bool   `json:"authPresent"`
	LastError   string `json:"lastError,omitempty"`
}

// ScanStatus is returned by GET /api/v1/scan.
type ScanStatus struct {
	Running    bool       `json:"running"`
	LastScan   *time.Time `json:"lastScan"`
	ImageCount int        `json:"imageCount"`
	Errors     []string   `json:"errors,omitempty"`
}

// ScanRequest scopes a manual scan. Empty fields mean a full scan.
type ScanRequest struct {
	Namespace    string `json:"namespace,omitempty"`
	WorkloadKind string `json:"workloadKind,omitempty"`
	WorkloadName string `json:"workloadName,omitempty"`
}

// Settings mirrors the configurable options.
type Settings struct {
	ScanIntervalSeconds     int      `json:"scanIntervalSeconds"`
	IncludedNamespaces      []string `json:"includedNamespaces"`
	ExcludedNamespaces      []string `json:"excludedNamespaces"`
	IncludeCompletedPods    bool     `json:"includeCompletedPods"`
	ExcludeInternalRegistry bool     `json:"excludeInternalRegistry"`
	RegistryTimeoutSeconds  int      `json:"registryTimeoutSeconds"`
	Theme                   string   `json:"theme"` // system | light | dark
	ShortDigests            bool     `json:"shortDigests"`
}

// DefaultSettings returns safe defaults.
func DefaultSettings() Settings {
	return Settings{
		ScanIntervalSeconds:    28800, // 8 hours
		IncludedNamespaces:     nil,
		ExcludedNamespaces:     []string{"kube-system", "kube-public", "kube-node-lease", "openshift*"},
		IncludeCompletedPods:   false,
		RegistryTimeoutSeconds: 15,
		Theme:                  "system",
		ShortDigests:           true,
	}
}
