package config

import (
	"strings"
	"testing"

	"github.com/k0sproject/k0s/pkg/airgap"
)

func TestListK0sImages(t *testing.T) {
	original := airgap.GetImageURIs(RenderK0sConfig().Spec, true)
	if len(original) == 0 {
		t.Errorf("airgap.GetImageURIs() = %v, want not empty", original)
	}
	var foundKubeRouter, foundCNINode, foundKonnectivity, foundEnvoy bool
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
		if strings.Contains(image, "envoy-distroless") {
			foundEnvoy = true
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
	if !foundEnvoy {
		t.Errorf("airgap.GetImageURIs() = %v, want to contain envoy-distroless", original)
	}

	filtered := ListK0sImages(RenderK0sConfig())
	if len(filtered) == 0 {
		t.Errorf("ListK0sImages() = %v, want not empty", filtered)
	}

	// make sure the list includes all images from the metadata
	for _, i := range Metadata.Images {
		found := false
		for _, f := range filtered {
			if f == i.URI() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListK0sImages() = %v, want to contain %s", filtered, i.URI())
		}
	}

	// make sure images are proxied
	for _, image := range filtered {
		if !strings.HasPrefix(image, "proxy.replicated.com/anonymous/") {
			t.Errorf("ListK0sImages() = %v, want %s to be proxied", filtered, image)
		}
	}

	// make sure the list does not contain excluded images
	for _, image := range filtered {
		if strings.Contains(image, "kube-router") {
			t.Errorf("ListK0sImages() = %v, want not to contain kube-router", filtered)
		}
		if strings.Contains(image, "cni-node") {
			t.Errorf("ListK0sImages() = %v, want not to contain kube-router", filtered)
		}
		if strings.Contains(image, "apiserver-network-proxy-agent") {
			t.Errorf("ListK0sImages() = %v, want not to contain apiserver-network-proxy-agent", filtered)
		}
		if strings.Contains(image, "envoy-distroless") {
			t.Errorf("ListK0sImages() = %v, want not to contain envoy-distroless", filtered)
		}
	}
}
