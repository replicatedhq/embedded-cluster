package registry

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	imagetypes "github.com/containers/image/v5/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/tests/integration/kind/registry/static"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
)

func createHostPortService(t *testing.T, clusterName string, kubeconfig string) string {
	// create a Pod and PVC to test that the data dir is mounted
	b, err := static.FS.ReadFile("hostport.yaml")
	if err != nil {
		t.Fatalf("failed to read hostport.yaml: %s", err)
	}
	filename := util.WriteTempFile(t, "hostport-*.yaml", b, 0644)
	util.KubectlApply(t, kubeconfig, "registry", filename)

	registryPort := util.KindGetExposedPort(t, clusterName, "30500")
	return net.JoinHostPort("127.0.0.1", registryPort)
}

func copyImageToRegistry(t *testing.T, registryAddr string, image string) {
	parts := strings.Split(image, "/")
	src := fmt.Sprintf("docker://%s", image)
	dst := fmt.Sprintf("docker://%s/%s", registryAddr, parts[len(parts)-1])

	dstRef, err := alltransports.ParseImageName(dst)
	if err != nil {
		t.Fatalf("unable to parse destination image reference: %v", err)
	}

	srcRef, err := alltransports.ParseImageName(src)
	if err != nil {
		t.Fatalf("unable to parse source image reference: %v", err)
	}

	policyContext, err := getPolicyContext()
	if err != nil {
		t.Fatalf("unable to get policy context: %v", err)
	}

	t.Logf("copying image %s to %s", src, dst)

	_, err = copy.Image(t.Context(), policyContext, dstRef, srcRef, &copy.Options{
		SourceCtx: &imagetypes.SystemContext{
			ArchitectureChoice: helpers.ClusterArch(),
			OSChoice:           "linux",
		},
		DestinationCtx: &imagetypes.SystemContext{
			DockerInsecureSkipTLSVerify: imagetypes.OptionalBoolTrue,
			DockerAuthConfig: &imagetypes.DockerAuthConfig{
				Username: "embedded-cluster",
				Password: registry.GetRegistryPassword(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unable to copy image: %v", err)
	}
}

func runPodAndValidateImagePull(t *testing.T, kubeconfig string, podName string, fileName string) {
	b, err := static.FS.ReadFile(fileName)
	if err != nil {
		t.Fatalf("failed to read %s: %s", fileName, err)
	}
	filename := util.WriteTempFile(t, "pod-*.yaml", b, 0644)
	util.KubectlApply(t, kubeconfig, "kotsadm", filename)

	util.WaitForPodComplete(t, kubeconfig, "kotsadm", podName, 30*time.Second)
}

var imagePolicy = []byte(`{"default": [{"type": "insecureAcceptAnything"}]}`)

func getPolicyContext() (*signature.PolicyContext, error) {
	policy, err := signature.NewPolicyFromBytes(imagePolicy)
	if err != nil {
		return nil, fmt.Errorf("read default policy: %w", err)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, fmt.Errorf("create policy context: %w", err)
	}
	return policyContext, nil
}
