package constants

const (
	KotsadmNamespace         = "kotsadm"
	KotsadmServiceAccount    = "kotsadm"
	SeaweedFSNamespace       = "seaweedfs"
	RegistryNamespace        = "registry"
	VeleroNamespace          = "velero"
	EmbeddedClusterNamespace = "embedded-cluster"
)

const (
	EcRestoreStateCMName = "embedded-cluster-restore-state"
)

type InstallTarget string

const (
	InstallTargetLinux      InstallTarget = "linux"
	InstallTargetKubernetes InstallTarget = "kubernetes"
)
