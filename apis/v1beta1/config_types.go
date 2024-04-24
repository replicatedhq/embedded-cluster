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
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// BuildInExtension holds the override for a built-in extension (add-on). Name
// is matched to the extension name in the generated k0s configuration. Values
// property holds a string containig the `values.yaml` we want to use when
// merging with the default values.
type BuiltInExtension struct {
	Name   string `json:"name"`
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

type Extensions struct {
	Helm *k0sv1beta1.HelmExtensions `json:"helm,omitempty"`
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	Version              string               `json:"version,omitempty"`
	Roles                Roles                `json:"roles,omitempty"`
	UnsupportedOverrides UnsupportedOverrides `json:"unsupportedOverrides,omitempty"`
	Extensions           Extensions           `json:"extensions,omitempty"`
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

// OverrideForBuiltIn returns the override for the built-in extension with the
// given name. If no override is found an empty string is returned.
func (c *Config) OverrideForBuiltIn(bi string) string {
	for _, ext := range c.Spec.UnsupportedOverrides.BuiltInExtensions {
		if ext.Name != bi {
			continue
		}
		return ext.Values
	}
	return ""
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
