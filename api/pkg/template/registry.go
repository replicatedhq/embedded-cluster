package template

// Registry template functions

// hasLocalRegistry returns true if local registry is available (airgap installations)
func (e *Engine) hasLocalRegistry() bool {
	if e.registrySettings == nil {
		return false
	}
	return e.registrySettings.HasLocalRegistry
}

// localRegistryHost returns registry host with port (e.g., "10.128.0.11:5000")
func (e *Engine) localRegistryHost() string {
	if e.registrySettings == nil {
		return ""
	}
	return e.registrySettings.LocalRegistryHost
}

// localRegistryAddress returns full registry address with namespace (e.g., "10.128.0.11:5000/myapp")
func (e *Engine) localRegistryAddress() string {
	if e.registrySettings == nil {
		return ""
	}
	return e.registrySettings.LocalRegistryAddress
}

// localRegistryNamespace returns the app-specific namespace for registry isolation
func (e *Engine) localRegistryNamespace() string {
	if e.registrySettings == nil {
		return ""
	}
	return e.registrySettings.LocalRegistryNamespace
}

// imagePullSecretName returns the standardized image pull secret name
func (e *Engine) imagePullSecretName() string {
	if e.registrySettings == nil {
		return ""
	}
	return e.registrySettings.ImagePullSecretName
}

// localRegistryImagePullSecret returns the base64 encoded local registry or replicated registry/proxy image pull secret value
func (e *Engine) localRegistryImagePullSecret() string {
	if e.registrySettings == nil {
		return ""
	}
	return e.registrySettings.ImagePullSecretValue
}
