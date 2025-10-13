package types

// RegistrySettings contains all registry-related configuration for template functions
type RegistrySettings struct {
	// HasLocalRegistry indicates if a local registry is available (airgap installations)
	HasLocalRegistry bool `json:"hasLocalRegistry"`

	// Host is the registry host with port (e.g., "10.128.0.11:5000")
	Host string `json:"host"`

	// Address is the full registry address with namespace (e.g., "10.128.0.11:5000/myapp")
	Address string `json:"address"`

	// Namespace is the app-specific namespace for registry isolation
	Namespace string `json:"namespace"`

	// Username is the registry authentication username
	Username string `json:"username"`

	// Password is the registry authentication password
	Password string `json:"password"`

	// ImagePullSecretName is the standardized image pull secret name
	ImagePullSecretName string `json:"imagePullSecretName"`

	// ImagePullSecretValue is the base64 encoded local registry or replicated registry/proxy image pull secret value
	ImagePullSecretValue string `json:"imagePullSecretValue"`
}
