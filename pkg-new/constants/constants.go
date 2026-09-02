package constants

const (
	KotsadmServiceAccount    = "kotsadm"
	SeaweedFSNamespace       = "seaweedfs"
	RegistryNamespace        = "registry"
	VeleroNamespace          = "velero"
	EmbeddedClusterNamespace = "embedded-cluster"
)

const (
	EcRestoreStateCMName = "embedded-cluster-restore-state"

	// EmbeddedRegistryDataSourceAnnotation marks installation resources whose immutable registry
	// data is rebuilt from the airgap bundle instead of being restored from volume backups.
	EmbeddedRegistryDataSourceAnnotation   = "kots.io/embedded-registry-data-source"
	EmbeddedRegistryDataSourceAirgapBundle = "airgap-bundle"
)
