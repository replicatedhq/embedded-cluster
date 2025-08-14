package types

// RegistrySettings encapsulates all registry-related configuration
type RegistrySettings struct {
	// HasLocalRegistry is true when a local registry is available
	HasLocalRegistry bool
	// Host is the registry host with port (e.g., "10.96.0.10:5000")
	Host string
	// Namespace is the app slug for namespace isolation
	Namespace string
	// Address is the full address with namespace prefix
	Address string
	// ImagePullSecretName is the standardized secret name pattern
	ImagePullSecretName string
}
