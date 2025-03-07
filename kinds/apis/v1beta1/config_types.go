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
	"time"

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
	// WorkerProfiles holds the worker profiles used to configure the cluster.
	// The profile named "default" will be applied by default.
	WorkerProfiles []k0sv1beta1.WorkerProfile `json:"workerProfiles,omitempty"`
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
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	NodeCount     *NodeCount        `json:"nodeCount,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	WorkerProfile string            `json:"workerProfile,omitempty"`
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
	// A duration string is a sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	// +kubebuilder:validation:XIntOrString
	Timeout BackwardCompatibleDuration `json:"timeout,omitempty"`
	// ForceUpgrade when set to false, disables the use of the "--force" flag when upgrading the the chart (default: true).
	// +optional
	ForceUpgrade *bool `json:"forceUpgrade,omitempty"`
	// +kubebuilder:validation:Optional
	Order int `json:"order,omitempty"`
}

// BackwardCompatibleDuration is a metav1.Duration with a different JSON
// unmarshaler. The unmashaler accepts its value as either a string (e.g.
// 10m15s) or as an integer 64. If the value is of type integer then, for
// backward compatibility, it is interpreted as nano seconds.
type BackwardCompatibleDuration metav1.Duration

// MarshalJSON marshals the BackwardCompatibleDuration to integer nano seconds,
// the reverse of the k0s type.
func (b BackwardCompatibleDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Duration)
}

// UnmarshalJSON attempts unmarshals the provided value into a
// BackwardCompatibleDuration. This function attempts to unmarshal it as a
// string first and if that fails it attempts to parse it as an integer.
func (b *BackwardCompatibleDuration) UnmarshalJSON(data []byte) error {
	var duration metav1.Duration
	ustrerr := duration.UnmarshalJSON(data)
	if ustrerr == nil {
		*b = BackwardCompatibleDuration(duration)
		return nil
	}

	var integer int64
	if err := json.Unmarshal(data, &integer); err != nil {
		// we return the error from the first unmarshal attempt.
		return ustrerr
	}
	metadur := metav1.Duration{Duration: time.Duration(integer)}
	*b = BackwardCompatibleDuration(metadur)
	return nil
}

// Helm contains helm extension settings
type Helm struct {
	// +kubebuilder:validation:Optional
	ConcurrencyLevel int `json:"concurrencyLevel"`
	// +kubebuilder:validation:Optional
	Repositories []k0sv1beta1.Repository `json:"repositories"`
	// +kubebuilder:validation:Optional
	Charts []Chart `json:"charts"`
}

type Extensions struct {
	Helm *Helm `json:"helm,omitempty"`
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	Version string `json:"version,omitempty"`
	// V2Enabled is a temporary property that can be used to opt-in to the new installer. If set,
	// in addition to using the new v2 install method, v1 installations will be migrated to v2 on
	// upgrade. This property will be removed once the new installer is fully implemented and the
	// old installer is removed.
	V2Enabled            bool                 `json:"v2Enabled,omitempty"`
	BinaryOverrideURL    string               `json:"binaryOverrideUrl,omitempty"`
	MetadataOverrideURL  string               `json:"metadataOverrideUrl,omitempty"`
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
