package paths

import (
	"path/filepath"
)

// TmpSubDir returns the path to the tmp directory where embedded-cluster stores temporary files.
func TmpSubDir(homeDir string) string {
	return filepath.Join(homeDir, "tmp")
}

// BinsSubDir returns the path to the directory where embedded-cluster binaries are stored.
func BinsSubDir(homeDir string) string {
	return filepath.Join(homeDir, "bin")
}

// ChartsSubDir returns the path to the directory where embedded-cluster helm charts are stored.
func ChartsSubDir(homeDir string) string {
	return filepath.Join(homeDir, "charts")
}

// ImagesSubDir returns the path to the directory where docker images are stored.
func ImagesSubDir(homeDir string) string {
	return filepath.Join(homeDir, "images")
}

// K0sSubDir returns the path to the directory where k0s data is stored.
func K0sSubDir(homeDir string) string {
	return filepath.Join(homeDir, "k0s")
}

// SeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func SeaweedfsSubDir(homeDir string) string {
	return filepath.Join(homeDir, "seaweedfs")
}

// OpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func OpenEBSLocalSubDir(homeDir string) string {
	return filepath.Join(homeDir, "openebs-local")
}

// SupportSubDir returns the path to the directory where embedded-cluster support files are stored.
func SupportSubDir(homeDir string) string {
	return filepath.Join(homeDir, "support")
}

// EmbeddedClusterBinaryPath returns the full path to a materialized binary that belongs to embedded-cluster.
func EmbeddedClusterBinaryPath(homeDir string, name string) string {
	return filepath.Join(BinsSubDir(homeDir), name)
}

// KubeConfigPath returns the path to the kubeconfig file.
func KubeConfigPath(homeDir string) string {
	return filepath.Join(K0sSubDir(homeDir), "pki/admin.conf")
}

// KubeletConfigPath returns the path to the kubelet config file.
func KubeletConfigPath(homeDir string) string {
	return filepath.Join(K0sSubDir(homeDir), "kubelet.conf")
}

// SupportFilePath returns the full path to a materialized support file.
func SupportFilePath(homeDir string, name string) string {
	return filepath.Join(SupportSubDir(homeDir), name)
}
