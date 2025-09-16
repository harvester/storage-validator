package api

type Configuration struct {
	// Namespace to run checks in. Else use default from current context
	Namespace string `json:"namespace,omitempty"`

	// ImageURL to use to create a virtualmachineimage.
	// required to ensure check can be triggered
	ImageURL string `json:"imageURL"`

	// StorageClass to be used for storagechecks
	StorageClass string `json:"storageClass,omitempty"`

	// SnapshotClass associated with StorageClass to be used for running snapshot validation tests. If one is not provided we try and lookup from storageprofiles
	SnapshotClass string `json:"snapshotClass,omitempty"`

	// Override default VMSpec used for validating storage
	VMConfig VMSpec `json:"vmConfig,omitempty"`
	// SkipCleanup of resources created during validation
	SkipCleanup *bool `json:"skipCleanup,omitempty"`
	// Timeout represents time duration in seconds to wait before triggering cleanup
	Timeout *int `json:"timeout,omitempty"`
}

type VMSpec struct {
	CPU      uint32 `json:"cpu,omitempty"`
	Memory   string `json:"ram,omitempty"`
	DiskSize string `json:"diskSize,omitempty"`
}
