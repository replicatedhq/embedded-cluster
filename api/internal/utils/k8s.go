package utils

func GetK8sObjectMetaLabels(appSlug string, versionLabel string, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       appSlug,
		"app.kubernetes.io/version":    versionLabel,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/part-of":    "embedded-cluster",
		"app.kubernetes.io/managed-by": "embedded-cluster-installer",
	}
}
