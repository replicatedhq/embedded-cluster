package paths

import (
	"path/filepath"
)

// TmpSubDir returns the path to the tmp directory where embedded-cluster stores temporary files.
func TmpSubDir(dataDir string) string {
	return filepath.Join(dataDir, "tmp")
}

// BinsSubDir returns the path to the directory where embedded-cluster binaries are stored.
func BinsSubDir(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

// ChartsSubDir returns the path to the directory where embedded-cluster helm charts are stored.
func ChartsSubDir(dataDir string) string {
	return filepath.Join(dataDir, "charts")
}

// ImagesSubDir returns the path to the directory where docker images are stored.
func ImagesSubDir(dataDir string) string {
	return filepath.Join(dataDir, "images")
}

// K0sSubDir returns the path to the directory where k0s data is stored.
func K0sSubDir(dataDir string) string {
	return filepath.Join(dataDir, "k0s")
}

// SeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func SeaweedfsSubDir(dataDir string) string {
	return filepath.Join(dataDir, "seaweedfs")
}

// OpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func OpenEBSLocalSubDir(dataDir string) string {
	return filepath.Join(dataDir, "openebs-local")
}

// SupportSubDir returns the path to the directory where embedded-cluster support files are stored.
func SupportSubDir(dataDir string) string {
	return filepath.Join(dataDir, "support")
}

// PathToECBinary returns the full path to a materialized binary that belongs to embedded-cluster.
func PathToECBinary(dataDir string, name string) string {
	return filepath.Join(BinsSubDir(dataDir), name)
}

// PathToKubeConfig returns the path to the kubeconfig file.
func PathToKubeConfig(k0sDataDir string) string {
	return filepath.Join(k0sDataDir, "pki/admin.conf")
}

// PathToKubeletConfig returns the path to the kubelet config file.
func PathToKubeletConfig(k0sDataDir string) string {
	return filepath.Join(k0sDataDir, "kubelet.conf")
}

// PathToSupportFile returns the full path to a materialized support file.
func PathToSupportFile(dataDir string, name string) string {
	return filepath.Join(SupportSubDir(dataDir), name)
}

// EmbeddedClusterLogsSubDir returns the path to the directory where embedded-cluster logs
// are stored.
func EmbeddedClusterLogsSubDir() string {
	return "/var/log/embedded-cluster"
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func PathToLog(name string) string {
	return filepath.Join(EmbeddedClusterLogsSubDir(), name)
}

// K0sBinaryPath returns the path to the k0s binary when it is installed on the node. This
// does not return the binary just after we materialized it but the path we want it to be
// once it is installed.
func K0sBinaryPath() string {
	return "/usr/local/bin/k0s"
}

// PathToK0sStatusSocket returns the full path to the k0s status socket.
func PathToK0sStatusSocket() string {
	return "/run/k0s/status.sock"
}

// PathToK0sConfig returns the full path to the k0s configuration file.
func PathToK0sConfig() string {
	return "/etc/k0s/k0s.yaml"
}

// PathToK0sContainerdConfig returns the full path to the k0s containerd configuration directory
func PathToK0sContainerdConfig() string {
	return "/etc/k0s/containerd.d/"
}

// PathToECConfig returns the full path to the embedded cluster configuration file.
// This file is used to specify the embedded cluster data directory.
func PathToECConfig() string {
	return "/etc/embedded-cluster/ec.yaml"
}
