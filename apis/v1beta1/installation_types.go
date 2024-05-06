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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	InstallationStateHelmChartUpdateFailure string = "HelmChartUpdateFailure"
	InstallationStateObsolete               string = "Obsolete"
	InstallationStateFailed                 string = "Failed"
	InstallationStateUnknown                string = "Unknown"
	InstallationStatePendingChartCreation   string = "PendingChartCreation"
)

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
	Images                  string `json:"images"`
	HelmCharts              string `json:"helmCharts"`
	EmbeddedClusterBinary   string `json:"embeddedClusterBinary"`
	EmbeddedClusterMetadata string `json:"embeddedClusterMetadata"`
}

// LicenseInfo holds information about the license used to install the cluster.
type LicenseInfo struct {
	IsSnapshotSupported bool `json:"isSnapshotSupported"`
}

// InstallationSpec defines the desired state of Installation.
type InstallationSpec struct {
	// ClusterID holds the cluster, generated during the installation.
	ClusterID string `json:"clusterID,omitempty"`
	// MetricsBaseURL holds the base URL for the metrics server.
	MetricsBaseURL string `json:"metricsBaseURL,omitempty"`
	// AirGap indicates if the installation is airgapped.
	AirGap bool `json:"airGap"`
	// Artifacts holds the location of the airgap bundle.
	Artifacts *ArtifactsLocation `json:"artifacts,omitempty"`
	// Config holds the configuration used at installation time.
	Config *ConfigSpec `json:"config,omitempty"`
	// EndUserK0sConfigOverrides holds the end user k0s config overrides
	// used at installation time.
	EndUserK0sConfigOverrides string `json:"endUserK0sConfigOverrides,omitempty"`
	// BinaryName holds the name of the binary used to install the cluster.
	// this will follow the pattern 'appslug-channelslug'
	BinaryName string `json:"binaryName,omitempty"`
	// LicenseInfo holds information about the license used to install the cluster.
	LicenseInfo *LicenseInfo `json:"licenseInfo,omitempty"`
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
}

// SetState sets the installation state and reason.
func (s *InstallationStatus) SetState(state string, reason string) {
	s.State = state
	s.Reason = reason
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
