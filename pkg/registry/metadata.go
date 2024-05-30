package registry

import (
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"sigs.k8s.io/yaml"
)

func getSeaweedfsNamespaceFromMetadata(metadata *ectypes.ReleaseMetadata) (string, error) {
	chart, err := getSeaweedfsChartFromMetadata(metadata)
	if err != nil {
		return "", fmt.Errorf("get seaweedfs charts settings from metadata: %w", err)
	}
	return chart.TargetNS, nil
}

func getSeaweedfsS3SecretNameFromMetadata(metadata *ectypes.ReleaseMetadata) (string, error) {
	chart, err := getSeaweedfsChartFromMetadata(metadata)
	if err != nil {
		return "", fmt.Errorf("get seaweedfs chart from metadata: %w", err)
	}
	var valuesStruct struct {
		Filer struct {
			S3 struct {
				ExistingConfigSecret string `json:"existingConfigSecret"`
			} `json:"s3"`
		} `json:"filer"`
	}
	err = yaml.Unmarshal([]byte(chart.Values), &valuesStruct)
	if err != nil {
		return "", fmt.Errorf("unmarshal chart values: %w", err)
	}
	if valuesStruct.Filer.S3.ExistingConfigSecret == "" {
		return "", fmt.Errorf("secret ref not found")
	}
	return valuesStruct.Filer.S3.ExistingConfigSecret, nil
}

func getSeaweedfsChartFromMetadata(metadata *ectypes.ReleaseMetadata) (*k0sv1beta1.Chart, error) {
	config, ok := metadata.BuiltinConfigs["seaweedfs"]
	if !ok {
		return nil, fmt.Errorf("config not found")
	}
	if len(config.Charts) == 0 {
		return nil, fmt.Errorf("chart not found")
	}
	return &config.Charts[0], nil
}

func getRegistryNamespaceFromMetadata(metadata *ectypes.ReleaseMetadata) (string, error) {
	chart, err := getRegistryChartFromMetadata(metadata)
	if err != nil {
		return "", fmt.Errorf("get registry chart from metadata: %w", err)
	}
	return chart.TargetNS, nil
}

func getRegistryS3SecretNameFromMetadata(metadata *ectypes.ReleaseMetadata) (string, error) {
	chart, err := getRegistryChartFromMetadata(metadata)
	if err != nil {
		return "", fmt.Errorf("get registry chart from metadata: %w", err)
	}
	var valuesStruct struct {
		Secrets struct {
			S3 struct {
				SecretRef string `json:"secretRef"`
			} `json:"s3"`
		} `json:"secrets"`
	}
	err = yaml.Unmarshal([]byte(chart.Values), &valuesStruct)
	if err != nil {
		return "", fmt.Errorf("unmarshal chart values: %w", err)
	}
	if valuesStruct.Secrets.S3.SecretRef == "" {
		return "", fmt.Errorf("secret ref not found")
	}
	return valuesStruct.Secrets.S3.SecretRef, nil
}

func getRegistryChartFromMetadata(metadata *ectypes.ReleaseMetadata) (*k0sv1beta1.Chart, error) {
	config, ok := metadata.BuiltinConfigs["registry-ha"]
	if !ok {
		return nil, fmt.Errorf("config not found")
	}
	if len(config.Charts) == 0 {
		return nil, fmt.Errorf("chart not found")
	}
	return &config.Charts[0], nil
}
