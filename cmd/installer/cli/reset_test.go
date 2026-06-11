package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func newKubeletKubeconfig() *clientcmdapi.Config {
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["local"] = &clientcmdapi.Cluster{
		Server:                   "https://127.0.0.1:6443",
		CertificateAuthorityData: []byte("ca-cert-bytes"),
	}
	cfg.AuthInfos["kubelet"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: []byte("client-cert"),
		ClientKeyData:         []byte("client-key"),
	}
	cfg.Contexts["kubelet@local"] = &clientcmdapi.Context{
		Cluster:  "local",
		AuthInfo: "kubelet",
	}
	cfg.CurrentContext = "kubelet@local"
	return cfg
}

func TestBuildSATokenConfig_HappyPath(t *testing.T) {
	kubelet := newKubeletKubeconfig()
	const token = "sa-token-value"

	got, err := buildSATokenConfig(kubelet, token)
	require.NoError(t, err)

	require.Contains(t, got.Clusters, "default")
	assert.Equal(t, "https://127.0.0.1:6443", got.Clusters["default"].Server)
	assert.Equal(t, []byte("ca-cert-bytes"), got.Clusters["default"].CertificateAuthorityData)

	require.Contains(t, got.AuthInfos, "default")
	assert.Equal(t, token, got.AuthInfos["default"].Token)
	assert.Empty(t, got.AuthInfos["default"].ClientCertificateData, "must not carry kubelet client cert")
	assert.Empty(t, got.AuthInfos["default"].ClientKeyData, "must not carry kubelet client key")

	require.Contains(t, got.Contexts, "default")
	assert.Equal(t, "default", got.Contexts["default"].Cluster)
	assert.Equal(t, "default", got.Contexts["default"].AuthInfo)
	assert.Equal(t, "default", got.CurrentContext)
}

func TestBuildSATokenConfig_NoCurrentContext(t *testing.T) {
	kubelet := newKubeletKubeconfig()
	kubelet.CurrentContext = ""

	_, err := buildSATokenConfig(kubelet, "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current-context")
}

func TestBuildSATokenConfig_CurrentContextMissingFromContexts(t *testing.T) {
	kubelet := newKubeletKubeconfig()
	kubelet.CurrentContext = "does-not-exist"

	_, err := buildSATokenConfig(kubelet, "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}

func TestBuildSATokenConfig_ContextReferencesMissingCluster(t *testing.T) {
	kubelet := newKubeletKubeconfig()
	kubelet.Contexts["kubelet@local"].Cluster = "ghost-cluster"

	_, err := buildSATokenConfig(kubelet, "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost-cluster")
}

func TestBuildSATokenConfig_DeterministicAcrossMultiCluster(t *testing.T) {
	// Previously, picking a cluster via map iteration was non-deterministic.
	// Confirm the helper consistently selects the cluster referenced by
	// CurrentContext rather than any other cluster in the kubeconfig.
	kubelet := newKubeletKubeconfig()
	kubelet.Clusters["other"] = &clientcmdapi.Cluster{
		Server:                   "https://10.0.0.1:6443",
		CertificateAuthorityData: []byte("other-ca"),
	}

	for i := 0; i < 5; i++ {
		got, err := buildSATokenConfig(kubelet, "tok")
		require.NoError(t, err)
		assert.Equal(t, "https://127.0.0.1:6443", got.Clusters["default"].Server,
			"iteration %d picked the wrong cluster", i)
	}
}
