package types

// RegistrySettings encapsulates all registry-related configuration
type RegistrySettings struct {
	HasLocalRegistry    bool   // whether a local registry is available
	Host                string // e.g., "10.96.0.10:5000"
	Namespace           string // app slug for namespace isolation
	Address             string // full address with namespace prefix
	ImagePullSecretName string // standardized secret name pattern
}