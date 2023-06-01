package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CliSuite struct {
	common.FootlooseSuite
}

func (s *CliSuite) TestK0sCliInstallStopAndStart() {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err, "failed to SSH into controller")
	defer ssh.Disconnect()

	s.T().Run("k0sInstall", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		// Install with some arbitrary kubelet flags so we see those get properly passed to the kubelet
		_, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller --enable-worker --no-taints")
		assert.NoError(err)
		// assert.Equal("", out)

		require.NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl get namespaces -o json 2>/dev/null")
		require.NoError(err)

		namespaces := &K8sNamespaces{}
		assert.NoError(json.Unmarshal([]byte(output), namespaces))

		assert.Len(namespaces.Items, 4)
		assert.Equal("default", namespaces.Items[0].Metadata.Name)
		assert.Equal("kube-node-lease", namespaces.Items[1].Metadata.Name)
		assert.Equal("kube-public", namespaces.Items[2].Metadata.Name)
		assert.Equal("kube-system", namespaces.Items[3].Metadata.Name)

		kc, err := s.KubeClient(s.ControllerNode(0))
		require.NoError(err)

		err = s.WaitForNodeReady(s.ControllerNode(0), kc)
		assert.NoError(err)

		s.AssertSomeKubeSystemPods(kc)

		// Wait till we see all pods running, otherwise we get into weird timing issues and high probability of leaked
		// containerd shim processes
		require.NoError(common.WaitForDaemonSet(s.Context(), kc, "kube-proxy"))
		require.NoError(common.WaitForKubeRouterReady(s.Context(), kc))
		require.NoError(common.WaitForDeployment(s.Context(), kc, "coredns", "kube-system"))
	})

	s.T().Run("k0sStopStart", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		s.T().Log("waiting for k0s to terminate")
		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s stop")
		s.Require().NoError(err)
		_, err = ssh.ExecWithOutput(s.Context(), "while pidof k0s containerd kubelet; do sleep 0.1s; done")
		s.Require().NoError(err)

		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
		require.NoError(err)

		require.NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl get namespaces -o json 2>/dev/null")
		require.NoError(err)

		namespaces := &K8sNamespaces{}
		assert.NoError(json.Unmarshal([]byte(output), namespaces))

		assert.Len(namespaces.Items, 4)
		assert.Equal("default", namespaces.Items[0].Metadata.Name)
		assert.Equal("kube-node-lease", namespaces.Items[1].Metadata.Name)
		assert.Equal("kube-public", namespaces.Items[2].Metadata.Name)
		assert.Equal("kube-system", namespaces.Items[3].Metadata.Name)

		kc, err := s.KubeClient(s.ControllerNode(0))
		require.NoError(err)

		err = s.WaitForNodeReady(s.ControllerNode(0), kc)
		assert.NoError(err)

		s.AssertSomeKubeSystemPods(kc)

		// Wait till we see all pods running, otherwise we get into weird timing issues and high probability of leaked
		// containerd shim processes
		require.NoError(common.WaitForDaemonSet(s.Context(), kc, "kube-proxy"))
		require.NoError(common.WaitForKubeRouterReady(s.Context(), kc))
		require.NoError(common.WaitForDeployment(s.Context(), kc, "coredns", "kube-system"))
	})
}

func TestCliCommandSuite(t *testing.T) {
	s := CliSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}

type K8sNamespaces struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Labels            struct {
				KubernetesIoMetadataName string `json:"kubernetes.io/metadata.name"`
			} `json:"labels"`
			ManagedFields []struct {
				APIVersion string `json:"apiVersion"`
				FieldsType string `json:"fieldsType"`
				FieldsV1   struct {
					FMetadata struct {
						FLabels struct {
							FKubernetesIoMetadataName struct {
							} `json:"f:kubernetes.io/metadata.name"`
						} `json:"f:labels"`
					} `json:"f:metadata"`
				} `json:"fieldsV1"`
				Manager   string    `json:"manager"`
				Operation string    `json:"operation"`
				Time      time.Time `json:"time"`
			} `json:"managedFields"`
			Name            string `json:"name"`
			ResourceVersion string `json:"resourceVersion"`
			UID             string `json:"uid"`
		} `json:"metadata"`
		Spec struct {
			Finalizers []string `json:"finalizers"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
	Kind     string `json:"kind"`
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
		SelfLink        string `json:"selfLink"`
	} `json:"metadata"`
}
