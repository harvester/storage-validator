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

	// AccessMode is the PVC access mode used for all volumes created during the
	// run. Defaults to ReadWriteMany. Set to ReadWriteOnce for node-local /
	// RWO-only drivers (e.g. LVM CSI) that reject MULTI_NODE_MULTI_WRITER.
	AccessMode string `json:"accessMode,omitempty"`

	// SingleNode relaxes the pre-flight requirement of >=2 Ready nodes to >=1,
	// so the storage-focused checks can run on a single-node cluster. Implies
	// SkipMigration, since live migration requires a second node.
	SingleNode *bool `json:"singleNode,omitempty"`

	// SkipMigration skips the live-migration check. Use for node-local storage
	// (e.g. LVM CSI) whose volumes are topology-pinned and cannot migrate.
	SkipMigration *bool `json:"skipMigration,omitempty"`
}

type VMSpec struct {
	CPU      uint32 `json:"cpu,omitempty"`
	Memory   string `json:"ram,omitempty"`
	DiskSize string `json:"diskSize,omitempty"`
}
