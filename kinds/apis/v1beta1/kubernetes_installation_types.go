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

type KubernetesInstallationState string

// What follows is a list of all valid states for an KubernetesInstallation object.
const (
	KubernetesInstallationStateEnqueued         KubernetesInstallationState = "Enqueued"
	KubernetesInstallationStateInstalling       KubernetesInstallationState = "Installing"
	KubernetesInstallationStateInstalled        KubernetesInstallationState = "Installed"
	KubernetesInstallationStateAddonsInstalling KubernetesInstallationState = "AddonsInstalling"
	KubernetesInstallationStateAddonsInstalled  KubernetesInstallationState = "AddonsInstalled"
	KubernetesInstallationStateObsolete         KubernetesInstallationState = "Obsolete"
	KubernetesInstallationStateFailed           KubernetesInstallationState = "Failed"
	KubernetesInstallationStateUnknown          KubernetesInstallationState = "Unknown"
)

// KubernetesInstallationSpec defines the desired state of KubernetesInstallation.
type KubernetesInstallationSpec struct {
	// ClusterID holds the cluster id, generated during the installation.
	ClusterID string `json:"clusterID,omitempty"`
	// MetricsBaseURL holds the base URL for the metrics server.
	MetricsBaseURL string `json:"metricsBaseURL,omitempty"`
	// Config holds the configuration used at installation time.
	Config *ConfigSpec `json:"config,omitempty"`
	// BinaryName holds the name of the binary used to install the cluster.
	// this will follow the pattern 'appslug-channelslug'
	BinaryName string `json:"binaryName,omitempty"`
	// LicenseInfo holds information about the license used to install the cluster.
	LicenseInfo *LicenseInfo `json:"licenseInfo,omitempty"`
	// Proxy holds the proxy configuration.
	Proxy *ProxySpec `json:"proxy,omitempty"`
	// AdminConsole holds the Admin Console configuration.
	AdminConsole AdminConsoleSpec `json:"adminConsole,omitempty"`
	// Manager holds the Manager configuration.
	Manager ManagerSpec `json:"manager,omitempty"`
	// HighAvailability indicates if the installation is high availability.
	HighAvailability bool `json:"highAvailability,omitempty"`
	// AirGap indicates if the installation is airgapped.
	AirGap bool `json:"airGap,omitempty"`
}

// KubernetesInstallationStatus defines the observed state of KubernetesInstallation
type KubernetesInstallationStatus struct {
	// State holds the current state of the installation.
	State KubernetesInstallationState `json:"state,omitempty"`
	// Reason holds the reason for the current state.
	Reason string `json:"reason,omitempty"`
}

// KubernetesInstallation is the Schema for the kubernetes installations API
type KubernetesInstallation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesInstallationSpec   `json:"spec,omitempty"`
	Status KubernetesInstallationStatus `json:"status,omitempty"`
}

func GetDefaultKubernetesInstallationSpec() KubernetesInstallationSpec {
	c := KubernetesInstallationSpec{}
	kubernetesInstallationSpecSetDefaults(&c)
	return c
}

func kubernetesInstallationSpecSetDefaults(c *KubernetesInstallationSpec) {
	adminConsoleSpecSetDefaults(&c.AdminConsole)
	managerSpecSetDefaults(&c.Manager)
}
