package types

// RegistrySettings contains all registry-related configuration for template functions
type RegistrySettings struct {
	// HasLocalRegistry indicates if a local registry is available (airgap installations)
	HasLocalRegistry bool `json:"hasLocalRegistry"`

	// LocalRegistryHost is the registry host with port (e.g., "10.128.0.11:5000")
	LocalRegistryHost string `json:"host"`

	// LocalRegistryAddress is the full registry address with namespace (e.g., "10.128.0.11:5000/myapp")
	LocalRegistryAddress string `json:"address"`

	// LocalRegistryNamespace is the app-specific namespace for registry isolation
	LocalRegistryNamespace string `json:"namespace"`

	// ImagePullSecretName is the standardized image pull secret name
	ImagePullSecretName string `json:"imagePullSecretName"`

	// ImagePullSecretValue is the base64 encoded local registry or replicated registry/proxy image pull secret value
	ImagePullSecretValue string `json:"imagePullSecretValue"`
}
