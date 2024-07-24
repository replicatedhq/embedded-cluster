// Package embeddedclusteroperator manages the installation of the embedded cluster
// operator chart.
package embeddedclusteroperator

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gosimple/slug"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const releaseName = "embedded-cluster-operator"

var (
	//go:embed static/values.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
	// Overwritten by -ldflags in Makefile
	EmbeddedOperatorImageOverride = ""
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}

	helmValues = make(map[string]interface{})
	if err := yaml.Unmarshal(rawvalues, &helmValues); err != nil {
		panic(fmt.Sprintf("unable to unmarshal values: %v", err))
	}

	helmValues["kotsVersion"] = adminconsole.Metadata.Version
	helmValues["embeddedClusterVersion"] = defaults.Version
	helmValues["embeddedClusterK0sVersion"] = defaults.K0sVersion

	if EmbeddedOperatorImageOverride != "" {
		// split ImageOverride into the image and tag
		parts := strings.Split(EmbeddedOperatorImageOverride, ":")
		if len(parts) != 2 {
			panic(fmt.Sprintf("invalid image override: %s", EmbeddedOperatorImageOverride))
		}
		helmValues["image"] = map[string]interface{}{
			"repository": parts[0],
			"tag":        parts[1],
		}
	}
}

// EmbeddedClusterOperator manages the installation of the embedded cluster operator
// helm chart.
type EmbeddedClusterOperator struct {
	namespace       string
	deployName      string
	endUserConfig   *embeddedclusterv1beta1.Config
	licenseFile     string
	airgap          bool
	releaseMetadata *types.ReleaseMetadata
	proxyEnv        map[string]string
	net             *embeddedclusterv1beta1.NetworkSpec
}

// Version returns the version of the embedded cluster operator chart.
func (e *EmbeddedClusterOperator) Version() (map[string]string, error) {
	return map[string]string{
		"EmbeddedClusterOperator": "v" + Metadata.Version,
	}, nil
}

func (a *EmbeddedClusterOperator) Name() string {
	return "EmbeddedClusterOperator"
}

// HostPreflights returns the host preflight objects found inside the EmbeddedClusterOperator
// Helm Chart, this is empty as there is no host preflight on there.
func (e *EmbeddedClusterOperator) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (e *EmbeddedClusterOperator) GetProtectedFields() map[string][]string {
	protectedFields := []string{"embeddedBinaryName", "embeddedClusterID"}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the embedded cluster operator chart.
func (e *EmbeddedClusterOperator) GenerateHelmConfig(onlyDefaults bool) ([]embeddedclusterv1beta1.Chart, []embeddedclusterv1beta1.Repository, error) {
	chartConfig := embeddedclusterv1beta1.Chart{
		Name:      releaseName,
		ChartName: Metadata.Location,
		Version:   Metadata.Version,
		TargetNS:  "embedded-cluster",
		Order:     3,
	}

	if !onlyDefaults {
		helmValues["embeddedBinaryName"] = defaults.BinaryName()
		helmValues["embeddedClusterID"] = metrics.ClusterID().String()
		if len(e.proxyEnv) > 0 {
			extraEnv := []map[string]interface{}{}
			for k, v := range e.proxyEnv {
				extraEnv = append(extraEnv, map[string]interface{}{
					"name":  k,
					"value": v,
				})
			}
			helmValues["extraEnv"] = extraEnv
		}
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)
	return []embeddedclusterv1beta1.Chart{chartConfig}, nil, nil
}

func (e *EmbeddedClusterOperator) GetAdditionalImages() []string {
	if tag, ok := Metadata.Images["docker.io/library/busybox"]; ok {
		return []string{fmt.Sprintf("proxy.replicated.com/anonymous/busybox:%s", tag)}
	}
	return nil
}

// createVersionMetadataConfigMap creates a ConfigMap with the version metadata for the embedded cluster operator.
func (e *EmbeddedClusterOperator) createVersionMetadataConfigmap(ctx context.Context, client client.Client) error {
	data, err := json.Marshal(e.releaseMetadata)
	if err != nil {
		return fmt.Errorf("unable to marshal release metadata: %w", err)
	}

	// we trim out the prefix v from the version and then slugify it, we use
	// the result as a suffix for the config map name.
	slugver := slug.Make(strings.TrimPrefix(defaults.Version, "v"))
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("version-metadata-%s", slugver),
			Namespace: e.namespace,
			Labels: map[string]string{
				"replicated.com/disaster-recovery": "ec-install",
			},
		},
		Data: map[string]string{
			"metadata.json": string(data),
		},
	}

	if err := client.Create(ctx, configmap); err != nil {
		return fmt.Errorf("unable to create version metadata config map: %w", err)
	}
	return nil
}

// Outro is executed after the cluster deployment. Waits for the embedded cluster operator
// to finish its deployment, creates the version metadata configmap (if in airgap) and
// the installation object.
func (e *EmbeddedClusterOperator) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	loading.Infof("Waiting for Embedded Cluster Operator to be ready")
	if err := kubeutils.WaitForDeployment(ctx, cli, e.namespace, e.deployName); err != nil {
		loading.Close()
		return err
	}
	loading.Closef("Embedded Cluster Operator is ready!")

	if e.releaseMetadata != nil {
		if err := e.createVersionMetadataConfigmap(ctx, cli); err != nil {
			return fmt.Errorf("unable to create version metadata: %w", err)
		}
	}

	cfg, err := release.GetEmbeddedClusterConfig()
	if err != nil {
		return err
	}
	var cfgspec *embeddedclusterv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}
	var euOverrides string
	if e.endUserConfig != nil {
		euOverrides = e.endUserConfig.Spec.UnsupportedOverrides.K0s
	}
	var license *kotsv1beta1.License
	if e.licenseFile != "" {
		l, err := helpers.ParseLicense(e.licenseFile)
		if err != nil {
			return fmt.Errorf("unable to parse license: %w", err)
		}
		license = l
	}

	// Configure proxy
	var proxySpec *embeddedclusterv1beta1.ProxySpec
	if len(e.proxyEnv) > 0 {
		proxySpec = &embeddedclusterv1beta1.ProxySpec{
			HTTPProxy:  e.proxyEnv["HTTP_PROXY"],
			HTTPSProxy: e.proxyEnv["HTTPS_PROXY"],
			NoProxy:    e.proxyEnv["NO_PROXY"],
		}
	}

	installation := embeddedclusterv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: time.Now().Format("20060102150405"),
			Labels: map[string]string{
				"replicated.com/disaster-recovery": "ec-install",
			},
		},
		Spec: embeddedclusterv1beta1.InstallationSpec{
			ClusterID:                 metrics.ClusterID().String(),
			MetricsBaseURL:            metrics.BaseURL(license),
			AirGap:                    e.airgap,
			Proxy:                     proxySpec,
			Network:                   e.net,
			Config:                    cfgspec,
			EndUserK0sConfigOverrides: euOverrides,
			BinaryName:                defaults.BinaryName(),
			LicenseInfo: &embeddedclusterv1beta1.LicenseInfo{
				IsDisasterRecoverySupported: licenseDisasterRecoverySupported(license),
			},
		},
	}
	if err := cli.Create(ctx, &installation); err != nil {
		return fmt.Errorf("unable to create installation: %w", err)
	}
	return nil
}

// New creates a new EmbeddedClusterOperator addon.
func New(endUserConfig *embeddedclusterv1beta1.Config, licenseFile string, airgapEnabled bool, releaseMetadata *types.ReleaseMetadata, proxyEnv map[string]string, net *embeddedclusterv1beta1.NetworkSpec) (*EmbeddedClusterOperator, error) {
	return &EmbeddedClusterOperator{
		namespace:       "embedded-cluster",
		deployName:      "embedded-cluster-operator",
		endUserConfig:   endUserConfig,
		licenseFile:     licenseFile,
		airgap:          airgapEnabled,
		releaseMetadata: releaseMetadata,
		proxyEnv:        proxyEnv,
		net:             net,
	}, nil
}

func licenseDisasterRecoverySupported(license *kotsv1beta1.License) bool {
	if license == nil {
		return false
	}
	return license.Spec.IsDisasterRecoverySupported
}
