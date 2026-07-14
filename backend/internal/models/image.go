package models

import "time"

// Status represents the comparison result for a container image.
type Status string

const (
	StatusUpToDate         Status = "up_to_date"
	StatusUpdateAvailable  Status = "update_available"
	StatusUnknown          Status = "unknown"
	StatusCheckFailed      Status = "check_failed"
)

// ImageRecord is the canonical data model for a discovered container image.
type ImageRecord struct {
	// ID is a stable composite key: namespace:workloadKind:workloadName:containerName
	ID string `json:"id"`

	Namespace     string `json:"namespace"`
	WorkloadKind  string `json:"workloadKind"`
	WorkloadName  string `json:"workloadName"`
	ContainerName string `json:"containerName"`

	// ConfiguredImage is the full image reference from the pod spec.
	ConfiguredImage string `json:"configuredImage"`
	Registry        string `json:"registry"`
	Repository      string `json:"repository"`
	Tag             string `json:"tag"`

	// RunningDigest is the digest of the image currently running in pods.
	RunningDigest string `json:"runningDigest"`
	// RegistryDigest is the digest the configured tag resolves to in the registry now.
	RegistryDigest string `json:"registryDigest"`

	Platform string `json:"platform"`
	Status   Status `json:"status"`

	// PodNames lists names of pods currently using this image.
	PodNames []string `json:"podNames"`

	LastChecked *time.Time `json:"lastChecked"`
	Error       string     `json:"error,omitempty"`
}

// Summary is the payload for GET /api/v1/summary.
type Summary struct {
	TotalImages       int        `json:"totalImages"`
	UpToDate          int        `json:"upToDate"`
	UpdatesAvailable  int        `json:"updatesAvailable"`
	Unknown           int        `json:"unknown"`
	CheckFailed       int        `json:"checkFailed"`
	UniqueRegistries  int        `json:"uniqueRegistries"`
	LastScan          *time.Time `json:"lastScan"`
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
	Running   bool       `json:"running"`
	LastScan  *time.Time `json:"lastScan"`
	ImageCount int       `json:"imageCount"`
	Errors    []string   `json:"errors,omitempty"`
}

// Settings mirrors the configurable options.
type Settings struct {
	ScanIntervalSeconds    int      `json:"scanIntervalSeconds"`
	IncludedNamespaces     []string `json:"includedNamespaces"`
	ExcludedNamespaces     []string `json:"excludedNamespaces"`
	IncludeCompletedPods   bool     `json:"includeCompletedPods"`
	RegistryTimeoutSeconds int      `json:"registryTimeoutSeconds"`
	Theme                  string   `json:"theme"` // system | light | dark
	ShortDigests           bool     `json:"shortDigests"`
}

// DefaultSettings returns safe defaults.
func DefaultSettings() Settings {
	return Settings{
		ScanIntervalSeconds:    28800, // 8 hours
		IncludedNamespaces:     nil,
		ExcludedNamespaces:     []string{"kube-system", "kube-public", "kube-node-lease"},
		IncludeCompletedPods:   false,
		RegistryTimeoutSeconds: 15,
		Theme:                  "system",
		ShortDigests:           true,
	}
}
