package config

import (
	"regexp"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/stretchr/testify/assert"
)

func TestListK0sImages(t *testing.T) {
	original := airgap.GetImageURIs(RenderK0sConfig("proxy.replicated.com").Spec, true)
	if len(original) == 0 {
		t.Errorf("airgap.GetImageURIs() = %v, want not empty", original)
	}
	var foundKubeRouter, foundCNINode, foundKonnectivity bool
	for _, image := range original {
		if strings.Contains(image, "kube-router") {
			foundKubeRouter = true
		}
		if strings.Contains(image, "cni-node") {
			foundCNINode = true
		}
		if strings.Contains(image, "apiserver-network-proxy-agent") {
			foundKonnectivity = true
		}
	}
	if !foundKubeRouter {
		t.Errorf("airgap.GetImageURIs() = %v, want to contain kube-router", original)
	}
	if !foundCNINode {
		t.Errorf("airgap.GetImageURIs() = %v, want to contain kube-router", original)
	}
	if !foundKonnectivity {
		t.Errorf("airgap.GetImageURIs() = %v, want to contain apiserver-network-proxy-agent", original)
	}

	filtered := ListK0sImages(RenderK0sConfig("proxy.replicated.com"))
	if len(filtered) == 0 {
		t.Errorf("ListK0sImages() = %v, want not empty", filtered)
	}

	// make sure the list includes all images from the metadata
	for _, i := range _metadata.Images {
		assert.Contains(t, filtered, i.String(), "image %s should be included", i.String())
	}

	// make sure images are proxied
	rx := regexp.MustCompile("proxy.replicated.com/(anonymous|library)/")
	for _, image := range filtered {
		assert.Regexp(t, rx, image, "image %s should be proxied", image)
	}

	// make sure the list does not contain excluded images
	for _, image := range filtered {
		assert.NotContains(t, image, "kube-router", "kube-router should be excluded")
		assert.NotContains(t, image, "cni-node", "cni-node should be excluded")
		assert.NotContains(t, image, "apiserver-network-proxy-agent", "apiserver-network-proxy-agent should be excluded")
	}
}
