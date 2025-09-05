package helm

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/distribution/reference"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	k8syaml "sigs.k8s.io/yaml"
)

type reducedResource struct {
	Kind string      `yaml:"kind"`
	Spec reducedSpec `yaml:"spec"`
}

type reducedSpec struct {
	Template       reducedTemplate    `yaml:"template"`
	Containers     []reducedContainer `yaml:"containers"`
	InitContainers []reducedContainer `yaml:"initContainers"`
}

type reducedTemplate struct {
	Spec reducedNestedSpec `yaml:"spec"`
}

type reducedNestedSpec struct {
	Containers     []reducedContainer `yaml:"containers"`
	InitContainers []reducedContainer `yaml:"initContainers"`
}

type reducedContainer struct {
	Image string `yaml:"image"`
}

func ExtractImagesFromChart(hcli Client, ref string, version string, values map[string]interface{}) ([]string, error) {
	parts := strings.Split(ref, "/")
	name := parts[len(parts)-1]

	manifests, err := hcli.Render(context.Background(), InstallOptions{
		ReleaseName:  name,
		ChartPath:    ref,
		ChartVersion: version,
		Values:       values,
		Namespace:    "default",
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	images := []string{}
	for i, manifest := range manifests {
		ims, err := extractImagesFromK8sManifest(manifest)
		if err != nil {
			return nil, fmt.Errorf("extract images from manifest %d: %w", i, err)
		}
		images = append(images, ims...)
	}

	images = helpers.UniqueStringSlice(images)
	sort.Strings(images)

	return images, nil
}

func extractImagesFromK8sManifest(resource []byte) ([]string, error) {
	images := []string{}

	r := reducedResource{}
	if err := k8syaml.Unmarshal([]byte(resource), &r); err != nil {
		// Not a k8s resource, ignore
		return nil, nil
	}

	for _, container := range r.Spec.Containers {
		if !slices.Contains(images, container.Image) {
			images = append(images, container.Image)
		}
	}

	for _, container := range r.Spec.Template.Spec.Containers {
		if !slices.Contains(images, container.Image) {
			images = append(images, container.Image)
		}
	}

	for _, container := range r.Spec.InitContainers {
		if !slices.Contains(images, container.Image) {
			images = append(images, container.Image)
		}
	}

	for _, container := range r.Spec.Template.Spec.InitContainers {
		if !slices.Contains(images, container.Image) {
			images = append(images, container.Image)
		}
	}

	for i, image := range images {
		// Normalize the image name to include docker.io and tag
		ref, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return nil, fmt.Errorf("parse image %s: %w", image, err)
		}
		images[i] = ref.String()
	}

	return images, nil
}
