package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestImageSubstitution(t *testing.T) {
	addon := &seaweedfs.SeaweedFS{
		DryRun:      true,
		ServiceCIDR: "10.96.0.0/16",
	}

	hcli, err := helm.NewClient(helm.HelmOptions{})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), t.Logf, nil, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "seaweedfs.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	// Build set of allowed images from metadata
	allowedImages := make(map[string]bool)
	for _, img := range seaweedfs.Metadata.Images {
		allowedImages[img.String()] = true
	}
	require.NotEmpty(t, allowedImages, "Metadata should contain at least one image")

	// Track all images found in manifests
	foundImages := make(map[string][]string) // map[image][]locations

	// Parse all manifests and extract images from any workload
	for _, manifest := range manifests {
		// Skip empty manifests
		if len(manifest) == 0 {
			continue
		}

		// Parse as unstructured to get Kind and Name
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal(manifest, &obj); err != nil {
			// Skip invalid manifests
			continue
		}

		kind := obj.GetKind()
		name := obj.GetName()

		// Skip non-workload resources
		if !isWorkloadKind(kind) {
			continue
		}

		// Extract pod template spec
		podSpec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "spec")
		if err != nil || !found {
			continue
		}

		// Convert to PodSpec for easier access
		podSpecBytes, err := yaml.Marshal(podSpec)
		if err != nil {
			continue
		}
		var ps corev1.PodSpec
		if err := yaml.Unmarshal(podSpecBytes, &ps); err != nil {
			continue
		}

		// Check all containers
		location := fmt.Sprintf("%s/%s", kind, name)
		for i, container := range ps.Containers {
			if container.Image != "" {
				containerLocation := fmt.Sprintf("%s.spec.containers[%d](%s)", location, i, container.Name)
				foundImages[container.Image] = append(foundImages[container.Image], containerLocation)
			}
		}

		// Check all init containers
		for i, container := range ps.InitContainers {
			if container.Image != "" {
				containerLocation := fmt.Sprintf("%s.spec.initContainers[%d](%s)", location, i, container.Name)
				foundImages[container.Image] = append(foundImages[container.Image], containerLocation)
			}
		}
	}

	require.NotEmpty(t, foundImages, "Should find at least one image in manifests")

	// Verify all found images are in the allowed list
	var unauthorizedImages []string
	for image, locations := range foundImages {
		if !allowedImages[image] {
			for _, loc := range locations {
				unauthorizedImages = append(unauthorizedImages, fmt.Sprintf("%s uses unauthorized image: %s", loc, image))
			}
		}

		// Additional checks for all images
		assert.NotContains(t, image, ":latest", "Image should not use :latest tag: %s", image)
		assert.Contains(t, image, "proxy.replicated.com/library", "Image should use proxy library registry: %s", image)
	}

	// Fail if any unauthorized images were found
	if len(unauthorizedImages) > 0 {
		t.Errorf("Found %d unauthorized images:\n%s", len(unauthorizedImages), strings.Join(unauthorizedImages, "\n"))
	}
}

// isWorkloadKind returns true if the kind can have a pod spec
func isWorkloadKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "ReplicaSet":
		return true
	default:
		return false
	}
}
