package registry

func applyRegistryLabels(labels map[string]string, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app"] = "docker-registry" // this is the backup/restore label for the registry
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}

func applySeaweedFSLabels(labels map[string]string, component string, excludeFromBackup bool) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	if excludeFromBackup {
		labels["velero.io/exclude-from-backup"] = "true"
	}
	labels["app.kubernetes.io/name"] = "seaweedfs" // this is the backup/restore label for seaweedfs
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/instance"] = "seaweedfs"
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}
