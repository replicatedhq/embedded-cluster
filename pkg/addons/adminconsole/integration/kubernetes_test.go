package integration

import (
	"context"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestKubernetes_Airgap(t *testing.T) {
	addon := &adminconsole.AdminConsole{
		DryRun: true,

		IsAirgap:           true,
		IsHA:               false,
		IsMultiNodeEnabled: true,
		Proxy:              nil,
		AdminConsolePort:   8080,

		Password:      "password",
		TLSCertBytes:  []byte("cert"),
		TLSKeyBytes:   []byte("key"),
		Hostname:      "admin-console",
		KotsInstaller: nil,
	}

	hcli, err := helm.NewClient(helm.HelmOptions{})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), t.Logf, nil, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "adminconsole.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	var adminDeployment *appsv1.Deployment
	for _, manifest := range manifests {
		manifestStr := string(manifest)
		// Look for the kotsadm deployment by its template source
		if strings.Contains(manifestStr, "# Source: admin-console/templates/kotsadm-deployment.yaml") {
			err := yaml.Unmarshal(manifest, &adminDeployment)
			require.NoError(t, err, "Failed to unmarshal Admin Console deployment")
			break
		}
	}

	require.NotNil(t, adminDeployment, "Admin Console deployment should not be nil")

	// Check for environment variables
	for _, env := range adminDeployment.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "EMBEDDED_CLUSTER_ID":
			assert.Fail(t, "EMBEDDED_CLUSTER_ID environment variable should not be set")
		case "EMBEDDED_CLUSTER_DATA_DIR":
			assert.Fail(t, "EMBEDDED_CLUSTER_DATA_DIR environment variable should not be set")
		case "EMBEDDED_CLUSTER_K0S_DIR":
			assert.Fail(t, "EMBEDDED_CLUSTER_K0S_DIR environment variable should not be set")
		case "ENABLE_IMPROVED_DR":
			assert.Fail(t, "ENABLE_IMPROVED_DR environment variable should not be set")
		}
	}

	for _, manifest := range manifests {
		manifestStr := string(manifest)
		if strings.Contains(manifestStr, "registry-creds") {
			assert.Fail(t, "registry-creds secret should not be created")
		}
	}
}
