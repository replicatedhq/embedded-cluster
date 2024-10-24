package configutils

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes/scheme"
)

// ValidateKotsConfigValues checks if the file exists and has the 'kots.io/v1beta1 ConfigValues' GVK
func ValidateKotsConfigValues(filename string) error {
	contents, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to read config values file: %w", err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, gvk, err := decode(contents, nil, nil)
	if err != nil {
		return fmt.Errorf("unable to decode config values file: %w", err)
	}

	if gvk.Group != "kots.io" || gvk.Version != "v1beta1" || gvk.Kind != "ConfigValues" {
		return fmt.Errorf("config values file is not a valid kots config values file")
	}

	return nil
}
