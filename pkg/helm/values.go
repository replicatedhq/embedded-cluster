package helm

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ohler55/ojg/jp"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/strvals"
	k8syaml "sigs.k8s.io/yaml"
)

// UnmarshalValues unmarshals the given JSON compatible YAML string into a map[string]interface{}.
func UnmarshalValues(valuesYaml string) (map[string]interface{}, error) {
	return chartutil.ReadValues([]byte(valuesYaml))
}

// MarshalValues marshals the given map[string]interface{} into a JSON compatible YAML string.
func MarshalValues(values map[string]interface{}) (string, error) {
	return chartutil.Values(values).YAML()
}

// SetValue sets the value at the given path in the values map. It uses the notation defined by
// helm "--set-json" flag.
func SetValue(values map[string]interface{}, path string, newValue interface{}) error {
	val, err := json.Marshal(newValue)
	if err != nil {
		return fmt.Errorf("parse value: %w", err)
	}
	s := fmt.Sprintf("%s=%s", path, val)
	err = strvals.ParseJSON(s, values)
	if err != nil {
		return fmt.Errorf("helm set json: %w", err)
	}
	return nil
}

// GetValue gets the value at the given JSON path in the values map.
func GetValue(values map[string]interface{}, path string) (interface{}, error) {
	x, err := jp.ParseString(path)
	if err != nil {
		return nil, fmt.Errorf("parse json path %q: %w", path, err)
	}
	v := x.Get(values)
	if len(v) == 0 {
		return nil, fmt.Errorf("value not found in path %q", path)
	}
	return v[0], nil
}

// PatchValues patches the values map with the given RFC6902 JSON patch.
func PatchValues(values map[string]interface{}, patchYAML string) (map[string]interface{}, error) {
	if len(patchYAML) == 0 {
		return values, nil
	}

	// convert original values to JSON
	originalYAML, err := MarshalValues(values)
	if err != nil {
		return nil, fmt.Errorf("marshal original values: %w", err)
	}
	originalJSON, err := k8syaml.YAMLToJSON([]byte(originalYAML))
	if err != nil {
		return nil, fmt.Errorf("convert original values to json: %w", err)
	}

	// convert patch to JSON
	patchJSON, err := k8syaml.YAMLToJSON([]byte(patchYAML))
	if err != nil {
		return nil, fmt.Errorf("convert patch to json: %w", err)
	}

	// apply as JSON merge patch
	resultJSON, err := jsonpatch.MergePatch(originalJSON, patchJSON)
	if err != nil {
		return nil, fmt.Errorf("patch values: %w", err)
	}

	// convert result back to YAML
	resultYAML, err := k8syaml.JSONToYAML(resultJSON)
	if err != nil {
		return nil, fmt.Errorf("convert patched values to yaml: %w", err)
	}

	// unmarshal result back to map
	result, err := UnmarshalValues(string(resultYAML))
	if err != nil {
		return nil, fmt.Errorf("unmarshal patched values: %w", err)
	}

	return result, nil
}
