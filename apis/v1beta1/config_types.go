/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"encoding/json"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "sigs.k8s.io/yaml"
)

// UnsupportedOverrides holds the config overrides used to configure
// the cluster.
type UnsupportedOverrides struct {
	// K0s holds the overrides used to configure k0s. These overrides
	// are merged on top of the default k0s configuration. As the data
	// layout inside this configuration is very dynamic we have chosen
	// to use a string here.
	K0s string `json:"k0s,omitempty"`
	// BuiltInExtensions holds overrides for the default add-ons we ship
	// with Embedded Cluster.
	BuiltInExtensions []BuiltInExtension `json:"builtInExtensions,omitempty"`
}

// BuiltInExtension holds the override for a built-in extension (add-on).
type BuiltInExtension struct {
	// The name of the helm chart to override values of, for instance `openebs`.
	Name string `json:"name"`
	// YAML-formatted helm values that will override those provided to the
	// chart by Embedded Cluster. Properties are overridden individually -
	// setting a new value for `images.tag` here will not prevent Embedded
	// Cluster from setting `images.pullPolicy = IfNotPresent`, for example.
	Values string `json:"values"`
}

// NodeRange contains a min and max or only one of them.
type NodeRange struct {
	// Min is the minimum number of nodes.
	Min *int `json:"min,omitempty"`
	// Max is the maximum number of nodes.
	Max *int `json:"max,omitempty"`
}

// NodeCount holds a series of rules for a given node role.
type NodeCount struct {
	// Values holds a list of allowed node counts.
	Values []int `json:"values,omitempty"`
	// NodeRange contains a min and max or only one of them (conflicts
	// with Values).
	Range *NodeRange `json:"range,omitempty"`
}

// NodeRole is the role of a node in the cluster.
type NodeRole struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	NodeCount   *NodeCount        `json:"nodeCount,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Roles is the various roles in the cluster.
type Roles struct {
	Controller NodeRole   `json:"controller,omitempty"`
	Custom     []NodeRole `json:"custom,omitempty"`
}

// Chart single helm addon
type Chart struct {
	Name      string `json:"name,omitempty"`
	ChartName string `json:"chartname,omitempty"`
	Version   string `json:"version,omitempty"`
	// +kubebuilder:validation:Optional
	Values   string `json:"values,omitempty"`
	TargetNS string `json:"namespace,omitempty"`
	// Timeout specifies the timeout for how long to wait for the chart installation to finish.
	// +kubebuilder:validation:Optional
	Timeout time.Duration `json:"timeout,omitempty"`
	// +kubebuilder:validation:Optional
	Order int `json:"order,omitempty"`
}

type Repository struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	CAFile   string `json:"caFile,omitempty"`
	CertFile string `json:"certFile,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
	KeyFile  string `json:"keyfile,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Helm contains helm extension settings
type Helm struct {
	// +kubebuilder:validation:Optional
	ConcurrencyLevel int `json:"concurrencyLevel"`
	// +kubebuilder:validation:Optional
	Repositories []Repository `json:"repositories"`
	// +kubebuilder:validation:Optional
	Charts []Chart `json:"charts"`
}

type Extensions struct {
	Helm *Helm `json:"helm,omitempty"`
}

func ConvertTo[T any](e Helm, t T) (T, error) {
	j, err := json.Marshal(e)
	if err != nil {
		return t, fmt.Errorf("unable to convert extensions: %w", err)
	}

	if err = json.Unmarshal(j, &t); err != nil {
		return t, fmt.Errorf("unable to unmarshal to new type: %w", err)
	}

	return t, nil
}

func ConvertFrom[T any](e k0sv1beta1.HelmExtensions, t T) (T, error) {
	j, err := json.Marshal(e)
	if err != nil {
		return t, fmt.Errorf("unable to convert extensions: %w", err)
	}

	if err = json.Unmarshal(j, &t); err != nil {
		return t, fmt.Errorf("unable to unmarshal to new type: %w", err)
	}

	return t, nil
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	Version              string               `json:"version,omitempty"`
	Roles                Roles                `json:"roles,omitempty"`
	UnsupportedOverrides UnsupportedOverrides `json:"unsupportedOverrides,omitempty"`
	Extensions           Extensions           `json:"extensions,omitempty"`
}

// OverrideForBuiltIn returns the override for the built-in extension with the
// given name. If no override is found an empty string is returned.
func (c ConfigSpec) OverrideForBuiltIn(bi string) string {
	for _, ext := range c.UnsupportedOverrides.BuiltInExtensions {
		if ext.Name != bi {
			continue
		}
		return ext.Values
	}
	return ""
}

// ApplyEndUserAddOnOverrides applies the end-user provided addon config on top
// of the provided addon configuration (cfg).
func (c *ConfigSpec) ApplyEndUserAddOnOverrides(name, cfg string) (string, error) {
	patch := c.OverrideForBuiltIn(name)
	if len(cfg) == 0 || len(patch) == 0 {
		if len(cfg) == 0 {
			return patch, nil
		}
		return cfg, nil
	}
	originalJSON, err := k8syaml.YAMLToJSON([]byte(cfg))
	if err != nil {
		return "", fmt.Errorf("unable to convert source yaml to json: %w", err)
	}
	patchJSON, err := k8syaml.YAMLToJSON([]byte(patch))
	if err != nil {
		return "", fmt.Errorf("unable to convert patch yaml to json: %w", err)
	}
	result, err := jsonpatch.MergePatch(originalJSON, patchJSON)
	if err != nil {
		return "", fmt.Errorf("unable to patch configuration: %w", err)
	}
	resultYAML, err := k8syaml.JSONToYAML(result)
	if err != nil {
		return "", fmt.Errorf("unable to convert result json to yaml: %w", err)
	}
	return string(resultYAML), nil
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
