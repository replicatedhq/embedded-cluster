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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// What follows is a list of all valid states for an Installation object.
const (
	InstallationStateWaiting                string = "Waiting"
	InstallationStateCopyingArtifacts       string = "CopyingArtifacts"
	InstallationStateEnqueued               string = "Enqueued"
	InstallationStateInstalling             string = "Installing"
	InstallationStateInstalled              string = "Installed"
	InstallationStateKubernetesInstalled    string = "KubernetesInstalled"
	InstallationStateAddonsInstalling       string = "AddonsInstalling"
	InstallationStateAddonsInstalled        string = "AddonsInstalled"
	InstallationStateHelmChartUpdateFailure string = "HelmChartUpdateFailure"
	InstallationStateObsolete               string = "Obsolete"
	InstallationStateFailed                 string = "Failed"
	InstallationStateUnknown                string = "Unknown"
	InstallationStatePendingChartCreation   string = "PendingChartCreation"
)

// Valid installation source types
const (
	InstallationSourceTypeCRD string = "CRD"
)

const (
	ConditionTypeV2MigrationInProgress = "V2MigrationInProgress"
)

// ConfigSecretEntryName holds the entry name we are looking for in the secret
// that holds the embedded cluster configuration.
const ConfigSecretEntryName = "config.yaml"

// NodeStatus is used to keep track of the status of a cluster node, we
// only hold its name and a hash of the node's status. Whenever the node
// status change we will be able to capture it and update the hash.
type NodeStatus struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// ArtifactsLocation defines a location from where we can download an
// airgap bundle. It contains individual URLs for each component of the
// bundle. These URLs are expected to point to a registry running inside
// the cluster, authentication for the registry is read from the cluster
// at execution time so they do not need to be provided here.
type ArtifactsLocation struct {
	Images                  string            `json:"images"`
	HelmCharts              string            `json:"helmCharts"`
	EmbeddedClusterBinary   string            `json:"embeddedClusterBinary"`
	EmbeddedClusterMetadata string            `json:"embeddedClusterMetadata"`
	AdditionalArtifacts     map[string]string `json:"additionalArtifacts,omitempty"`
}

// ProxySpec holds the proxy configuration.
type ProxySpec struct {
	HTTPProxy       string `json:"httpProxy,omitempty"`
	HTTPSProxy      string `json:"httpsProxy,omitempty"`
	ProvidedNoProxy string `json:"providedNoProxy,omitempty"`
	NoProxy         string `json:"noProxy,omitempty"`
}

// NetworkSpec holds the network configuration.
type NetworkSpec struct {
	NetworkInterface string `json:"networkInterface,omitempty"`
	GlobalCIDR       string `json:"globalCIDR,omitempty"`
	PodCIDR          string `json:"podCIDR,omitempty"`
	ServiceCIDR      string `json:"serviceCIDR,omitempty"`
	NodePortRange    string `json:"nodePortRange,omitempty"`
}

// AdminConsoleSpec holds the admin console configuration.
type AdminConsoleSpec struct {
	// Port holds the port on which the admin console will be served.
	Port int `json:"port,omitempty"`
}

// LocalArtifactMirrorSpec holds the local artifact mirror configuration.
type LocalArtifactMirrorSpec struct {
	// Port holds the port on which the local artifact mirror will be served.
	Port int `json:"port,omitempty"`
}

// ManagerSpec holds the manager configuration.
type ManagerSpec struct {
	// Port holds the port on which the manager will be served.
	Port int `json:"port,omitempty"`
}

// LicenseInfo holds information about the license used to install the cluster.
type LicenseInfo struct {
	IsDisasterRecoverySupported bool `json:"isDisasterRecoverySupported,omitempty"`
	IsMultiNodeEnabled          bool `json:"isMultiNodeEnabled,omitempty"`
}

// ConfigSecret holds a reference to secret containing the embedded cluster
// config. The config found on this secret overrides the configuration found
// in the InstallationSpec.
type ConfigSecret struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// InstallationSpec defines the desired state of Installation.
type InstallationSpec struct {
	// ClusterID holds the cluster, generated during the installation.
	ClusterID string `json:"clusterID,omitempty"`
	// MetricsBaseURL holds the base URL for the metrics server.
	MetricsBaseURL string `json:"metricsBaseURL,omitempty"`
	// Artifacts holds the location of the airgap bundle.
	Artifacts *ArtifactsLocation `json:"artifacts,omitempty"`
	// Config holds the configuration used at installation time.
	Config *ConfigSpec `json:"config,omitempty"`
	// BinaryName holds the name of the binary used to install the cluster.
	// this will follow the pattern 'appslug-channelslug'
	BinaryName string `json:"binaryName,omitempty"`
	// LicenseInfo holds information about the license used to install the cluster.
	LicenseInfo *LicenseInfo `json:"licenseInfo,omitempty"`
	// ConfigSecret holds a secret name and namespace. If this is set it means that
	// the Config for this Installation object must be read from there. This option
	// supersedes (overrides) the Config field.
	ConfigSecret *ConfigSecret `json:"configSecret,omitempty"`
	// SourceType indicates where this Installation object is stored (CRD, ConfigMap, etc...).
	SourceType string `json:"sourceType,omitempty"`

	// RuntimeConfig holds the runtime configuration used at installation time.
	RuntimeConfig *RuntimeConfigSpec `json:"runtimeConfig,omitempty"`

	// TODO: all fields below should be moved to RuntimeConfig

	// HighAvailability indicates if the installation is high availability.
	HighAvailability bool `json:"highAvailability,omitempty"`
	// AirGap indicates if the installation is airgapped.
	AirGap bool `json:"airGap,omitempty"`
	// EndUserK0sConfigOverrides holds the end user k0s config overrides
	// used at installation time.
	EndUserK0sConfigOverrides string `json:"endUserK0sConfigOverrides,omitempty"`

	Deprecated_Proxy               *ProxySpec               `json:"proxy,omitempty"`
	Deprecated_Network             *NetworkSpec             `json:"network,omitempty"`
	Deprecated_AdminConsole        *AdminConsoleSpec        `json:"adminConsole,omitempty"`
	Deprecated_LocalArtifactMirror *LocalArtifactMirrorSpec `json:"localArtifactMirror,omitempty"`
}

func (i *InstallationSpec) UnmarshalJSON(data []byte) error {
	type alias InstallationSpec
	ji := (*alias)(i)
	err := json.Unmarshal(data, &ji)
	if err != nil {
		return err
	}

	if i.SourceType == "" {
		i.SourceType = InstallationSourceTypeCRD
	}

	if i.Deprecated_Proxy != nil {
		if i.RuntimeConfig == nil {
			i.RuntimeConfig = &RuntimeConfigSpec{}
		}
		if i.RuntimeConfig.Proxy == nil {
			i.RuntimeConfig.Proxy = &ProxySpec{}
		}
		if i.Deprecated_Proxy.HTTPProxy != "" {
			i.RuntimeConfig.Proxy.HTTPProxy = i.Deprecated_Proxy.HTTPProxy
		}
		if i.Deprecated_Proxy.HTTPSProxy != "" {
			i.RuntimeConfig.Proxy.HTTPSProxy = i.Deprecated_Proxy.HTTPSProxy
		}
		if i.Deprecated_Proxy.NoProxy != "" {
			i.RuntimeConfig.Proxy.NoProxy = i.Deprecated_Proxy.NoProxy
		}
	}
	if i.Deprecated_Network != nil {
		if i.RuntimeConfig == nil {
			i.RuntimeConfig = &RuntimeConfigSpec{}
		}
		if i.Deprecated_Network.PodCIDR != "" {
			i.RuntimeConfig.Network.PodCIDR = i.Deprecated_Network.PodCIDR
		}
		if i.Deprecated_Network.ServiceCIDR != "" {
			i.RuntimeConfig.Network.ServiceCIDR = i.Deprecated_Network.ServiceCIDR
		}
		if i.Deprecated_Network.NodePortRange != "" {
			i.RuntimeConfig.Network.NodePortRange = i.Deprecated_Network.NodePortRange
		}
	}
	if i.Deprecated_AdminConsole != nil && i.Deprecated_AdminConsole.Port > 0 {
		if i.RuntimeConfig == nil {
			i.RuntimeConfig = &RuntimeConfigSpec{}
		}
		if i.RuntimeConfig.AdminConsole.Port == 0 {
			i.RuntimeConfig.AdminConsole.Port = i.Deprecated_AdminConsole.Port
		}
	}
	if i.Deprecated_LocalArtifactMirror != nil && i.Deprecated_LocalArtifactMirror.Port > 0 {
		if i.RuntimeConfig == nil {
			i.RuntimeConfig = &RuntimeConfigSpec{}
		}
		if i.RuntimeConfig.LocalArtifactMirror.Port == 0 {
			i.RuntimeConfig.LocalArtifactMirror.Port = i.Deprecated_LocalArtifactMirror.Port
		}
	}
	return nil
}

// ParseConfigSpecFromSecret reads the embedded cluster configuration from a secret.
// This function overrides the Config field in the InstallationSpec but does not
// save it to the cluster.
func (i *InstallationSpec) ParseConfigSpecFromSecret(secret corev1.Secret) error {
	data, ok := secret.Data[ConfigSecretEntryName]
	if !ok {
		return fmt.Errorf(
			"entry %s not found in secret %s/%s",
			ConfigSecretEntryName,
			secret.Namespace,
			secret.Name,
		)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config from secret: %w", err)
	}
	i.Config = &config.Spec
	return nil
}

// InstallationStatus defines the observed state of Installation
type InstallationStatus struct {
	// NodesStatus is a list of nodes and their status.
	NodesStatus []NodeStatus `json:"nodesStatus,omitempty"`
	// State holds the current state of the installation.
	State string `json:"state,omitempty"`
	// Reason holds the reason for the current state.
	Reason string `json:"reason,omitempty"`
	// PendingCharts holds the list of charts that are being created or updated.
	PendingCharts []string `json:"pendingCharts,omitempty"`

	// Conditions is an array of current observed installation conditions.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// SetState sets the installation state and reason.
func (s *InstallationStatus) SetState(state string, reason string, pendingCharts []string) {
	s.State = state
	s.Reason = reason
	s.PendingCharts = pendingCharts
}

// SetState sets the installation state and reason.
func (s *InstallationStatus) SetCondition(condition metav1.Condition) bool {
	return meta.SetStatusCondition(&s.Conditions, condition)
}

func (s *InstallationStatus) GetKubernetesInstalled() bool {
	if s.State == InstallationStateInstalled ||
		s.State == InstallationStateKubernetesInstalled ||
		s.State == InstallationStateAddonsInstalling ||
		s.State == InstallationStatePendingChartCreation ||
		s.State == InstallationStateHelmChartUpdateFailure {
		return true
	}
	return false
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="State of the installation"
//+kubebuilder:printcolumn:name="InstallerVersion",type="string",JSONPath=".spec.config.version",description="Installer version"
//+kubebuilder:printcolumn:name="CreatedAt",type="string",JSONPath=".metadata.creationTimestamp",description="Creation time of the installation"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Age of the resource"

// Installation is the Schema for the installations API
type Installation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstallationSpec   `json:"spec,omitempty"`
	Status InstallationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InstallationList contains a list of Installation
type InstallationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Installation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Installation{}, &InstallationList{})
}
