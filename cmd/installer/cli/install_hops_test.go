package cli

import (
	"os"
	"path/filepath"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/web"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	helmcli "helm.sh/helm/v3/pkg/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_buildRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		flags       *installFlags
		installCfg  *installConfig
		sslCertFile string
		wantErr     bool
		validate    func(*testing.T, runtimeconfig.RuntimeConfig)
	}{
		{
			name: "all ports and network settings set",
			flags: &installFlags{
				adminConsolePort:        8800,
				managerPort:             8801,
				localArtifactMirrorPort: 8802,
				proxySpec:               nil,
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  stringPtr("10.0.0.0/16"),
				},
			},
			sslCertFile: filepath.Join(tmpDir, "ca-certificates.crt"),
			installCfg:  &installConfig{},
			wantErr:     false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal(8800, rc.AdminConsolePort())
				req.Equal(8801, rc.ManagerPort())
				req.Equal(8802, rc.LocalArtifactMirrorPort())
				req.Equal("10.0.0.0/24", rc.PodCIDR())
				req.Equal("10.1.0.0/24", rc.ServiceCIDR())
				req.Equal("10.0.0.0/16", rc.GlobalCIDR())
				req.Equal(filepath.Join(tmpDir, "ca-certificates.crt"), rc.HostCABundlePath())
			},
		},
		{
			name: "with proxy spec",
			flags: &installFlags{
				adminConsolePort:        30000,
				managerPort:             30001,
				localArtifactMirrorPort: 30002,
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8080",
				},
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "192.168.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal(30000, rc.AdminConsolePort())
				req.Equal(30001, rc.ManagerPort())
				req.Equal(30002, rc.LocalArtifactMirrorPort())
				req.Equal("192.168.0.0/16", rc.PodCIDR())
				req.Equal("10.96.0.0/12", rc.ServiceCIDR())
				proxySpec := rc.ProxySpec()
				req.NotNil(proxySpec)
				req.Equal("http://proxy.example.com:8080", proxySpec.HTTPProxy)
				req.Equal("https://proxy.example.com:8080", proxySpec.HTTPSProxy)
			},
		},
		{
			name: "with global CIDR",
			flags: &installFlags{
				adminConsolePort:        8800,
				managerPort:             8801,
				localArtifactMirrorPort: 8802,
				proxySpec:               nil,
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  stringPtr("10.0.0.0/16"),
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/16", rc.GlobalCIDR())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			if tt.sslCertFile != "" {
				err := os.WriteFile(tt.sslCertFile, []byte("test cert"), 0644)
				require.NoError(t, err)

				t.Setenv("SSL_CERT_FILE", tt.sslCertFile)
				defer os.Unsetenv("SSL_CERT_FILE")
			}

			// Create a temporary directory for dataDir testing
			tt.flags.dataDir = tmpDir
			absoluteDataDir, err := filepath.Abs(tmpDir)
			req.NoError(err)

			rc := runtimeconfig.New(nil)

			err = buildRuntimeConfig(tt.flags, tt.installCfg, rc)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.Equal(absoluteDataDir, rc.Get().DataDir)
			req.NotEmpty(rc.HostCABundlePath())

			if tt.validate != nil {
				tt.validate(t, rc)
			}
		})
	}
}

func Test_buildKubernetesInstallation(t *testing.T) {
	tests := []struct {
		name     string
		flags    *installFlags
		wantErr  bool
		validate func(*testing.T, kubernetesinstallation.Installation)
	}{
		{
			name: "all values set",
			flags: &installFlags{
				adminConsolePort: 8800,
				managerPort:      8801,
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:       "http://proxy.example.com:8080",
					HTTPSProxy:      "https://proxy.example.com:8080",
					NoProxy:         "example.com,192.168.0.0/16",
					ProvidedNoProxy: "provided-no-proxy.example.com",
				},
				kubernetesEnvSettings: helmcli.New(),
			},
			wantErr: false,
			validate: func(t *testing.T, ki kubernetesinstallation.Installation) {
				req := require.New(t)
				req.Equal(8800, ki.AdminConsolePort())
				req.Equal(8801, ki.ManagerPort())
				proxySpec := ki.ProxySpec()
				req.NotNil(proxySpec)
				req.Equal("http://proxy.example.com:8080", proxySpec.HTTPProxy)
				req.Equal("https://proxy.example.com:8080", proxySpec.HTTPSProxy)
				req.Equal("example.com,192.168.0.0/16", proxySpec.NoProxy)
				req.Equal("provided-no-proxy.example.com", proxySpec.ProvidedNoProxy)
				req.NotNil(ki.GetKubernetesEnvSettings())
			},
		},
		{
			name: "minimal values",
			flags: &installFlags{
				adminConsolePort:      30000,
				managerPort:           30001,
				proxySpec:             nil,
				kubernetesEnvSettings: helmcli.New(),
			},
			wantErr: false,
			validate: func(t *testing.T, ki kubernetesinstallation.Installation) {
				req := require.New(t)
				req.Equal(30000, ki.AdminConsolePort())
				req.Equal(30001, ki.ManagerPort())
				req.Nil(ki.ProxySpec())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			ki := kubernetesinstallation.New(nil)

			err := buildKubernetesInstallation(tt.flags, ki)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)

			if tt.validate != nil {
				tt.validate(t, ki)
			}
		})
	}
}

func Test_buildMetricsReporter(t *testing.T) {
	tests := []struct {
		name       string
		cmd        *cobra.Command
		installCfg *installConfig
		validate   func(*testing.T, *installReporter)
	}{
		{
			name: "all values set with flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}
				cmd.Use = "install"
				cmd.Flags().String("flag1", "value1", "")
				cmd.Flags().String("flag2", "value2", "")
				return cmd
			}(),
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-123",
						AppSlug:   "my-app",
					},
				},
				clusterID: "cluster-456",
			},
			validate: func(t *testing.T, reporter *installReporter) {
				req := require.New(t)
				req.Equal("license-123", reporter.licenseID)
				req.Equal("my-app", reporter.appSlug)
				req.NotNil(reporter.reporter)
			},
		},
		{
			name: "minimal values without flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}
				cmd.Use = "install"
				return cmd
			}(),
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-789",
						AppSlug:   "simple-app",
					},
				},
				clusterID: "cluster-012",
			},
			validate: func(t *testing.T, reporter *installReporter) {
				req := require.New(t)
				req.Equal("license-789", reporter.licenseID)
				req.Equal("simple-app", reporter.appSlug)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := buildMetricsReporter(tt.cmd, tt.installCfg)

			if tt.validate != nil {
				tt.validate(t, reporter)
			}
		})
	}
}

func Test_buildAPIOptions(t *testing.T) {
	tests := []struct {
		name            string
		flags           installFlags
		installCfg      *installConfig
		rc              runtimeconfig.RuntimeConfig
		ki              kubernetesinstallation.Installation
		metricsReporter metrics.ReporterInterface
		wantErr         bool
		validate        func(*testing.T, apiOptions)
	}{
		{
			name: "all options set",
			flags: installFlags{
				adminConsolePassword: "password123",
				target:               "linux",
				hostname:             "example.com",
				managerPort:          8800,
				headless:             false,
				ignoreHostPreflights: true,
				airgapBundle:         "/path/to/bundle.airgap",
			},
			installCfg: &installConfig{
				licenseBytes: []byte("license-data"),
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
				clusterID:    "cluster-123",
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: &kotsv1beta1.Airgap{},
				},
				embeddedAssetsSize: 1024 * 1024,
				configValues: &kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"key1": {Value: "value1"},
							"key2": {Value: "value2"},
						},
					},
				},
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetAdminConsolePort(30303)
				return rc
			}(),
			ki: func() kubernetesinstallation.Installation {
				ki := kubernetesinstallation.New(nil)
				ki.SetAdminConsolePort(30304)
				return ki
			}(),
			metricsReporter: &metrics.MockReporter{},
			wantErr:         false,
			validate: func(t *testing.T, opts apiOptions) {
				req := require.New(t)
				req.Equal(apitypes.InstallTargetLinux, opts.InstallTarget)
				req.Equal("password123", opts.Password)
				req.NotEmpty(opts.PasswordHash)
				err := bcrypt.CompareHashAndPassword(opts.PasswordHash, []byte("password123"))
				req.NoError(err)
				req.Equal([]byte("cert-data"), opts.TLSConfig.CertBytes)
				req.Equal([]byte("key-data"), opts.TLSConfig.KeyBytes)
				req.Equal("example.com", opts.TLSConfig.Hostname)
				req.Equal([]byte("license-data"), opts.License)
				req.Equal("/path/to/bundle.airgap", opts.AirgapBundle)
				req.NotNil(opts.AirgapMetadata)
				req.Equal(int64(1024*1024), opts.EmbeddedAssetsSize)
				req.Equal(apitypes.AppConfigValues{"key1": {Value: "value1"}, "key2": {Value: "value2"}}, opts.ConfigValues)
				req.NotNil(opts.EndUserConfig)
				req.Equal("cluster-123", opts.ClusterID)
				req.Equal(apitypes.ModeInstall, opts.Mode)
				req.Equal(30303, opts.LinuxConfig.RuntimeConfig.AdminConsolePort())
				req.Equal(30304, opts.KubernetesConfig.Installation.AdminConsolePort())
				req.Equal(false, opts.RequiresInfraUpgrade)
				req.Equal(8800, opts.ManagerPort)
				req.Equal(false, opts.Headless)
				req.Equal(true, opts.LinuxConfig.AllowIgnoreHostPreflights)
				req.Equal(web.ModeInstall, opts.WebMode)
			},
		},
		{
			name: "minimal options",
			flags: installFlags{
				adminConsolePassword: "pass",
				target:               "kubernetes",
				hostname:             "",
				managerPort:          30000,
				headless:             true,
				ignoreHostPreflights: false,
			},
			installCfg: &installConfig{
				clusterID: "cluster-123",
			},
			rc:              runtimeconfig.New(nil),
			ki:              kubernetesinstallation.New(nil),
			metricsReporter: &metrics.MockReporter{},
			wantErr:         false,
			validate: func(t *testing.T, opts apiOptions) {
				req := require.New(t)
				req.Equal(apitypes.InstallTargetKubernetes, opts.InstallTarget)
				req.Equal("pass", opts.Password)
				req.NotEmpty(opts.PasswordHash)
				req.Equal("", opts.TLSConfig.Hostname)
				req.Equal(30000, opts.ManagerPort)
				req.Equal(true, opts.Headless)
				req.Equal(false, opts.LinuxConfig.AllowIgnoreHostPreflights)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts, err := buildAPIOptions(tt.flags, tt.installCfg, tt.rc, tt.ki, tt.metricsReporter)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}

func Test_buildHelmClientOptions(t *testing.T) {
	dataDir := t.TempDir()

	tests := []struct {
		name       string
		installCfg *installConfig
		rc         runtimeconfig.RuntimeConfig
		validate   func(*testing.T, runtimeconfig.RuntimeConfig, helm.HelmOptions)
	}{
		{
			name: "airgap mode",
			installCfg: &installConfig{
				isAirgap: true,
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetDataDir(dataDir)
				return rc
			}(),
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig, opts helm.HelmOptions) {
				req := require.New(t)
				req.Equal(rc.PathToEmbeddedClusterBinary("helm"), opts.HelmPath)
				req.NotNil(opts.KubernetesEnvSettings)
				req.NotEmpty(opts.AirgapPath)
			},
		},
		{
			name: "non-airgap mode",
			installCfg: &installConfig{
				isAirgap: false,
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetDataDir(dataDir)
				return rc
			}(),
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig, opts helm.HelmOptions) {
				req := require.New(t)
				req.Equal(rc.PathToEmbeddedClusterBinary("helm"), opts.HelmPath)
				req.NotNil(opts.KubernetesEnvSettings)
				req.Empty(opts.AirgapPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildHelmClientOptions(tt.installCfg, tt.rc)

			if tt.validate != nil {
				tt.validate(t, tt.rc, opts)
			}
		})
	}
}

func Test_buildKotsInstallOptions(t *testing.T) {
	tests := []struct {
		name             string
		installCfg       *installConfig
		flags            installFlags
		kotsadmNamespace string
		loading          *spinner.MessageWriter
		validate         func(*testing.T, kotscli.InstallOptions)
	}{
		{
			name: "all options set",
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug: "my-app",
					},
				},
				licenseBytes: []byte("license-data"),
				clusterID:    "test-cluster-id",
			},
			flags: installFlags{
				airgapBundle:        "/path/to/bundle.airgap",
				configValues:        "/path/to/config.yaml",
				ignoreAppPreflights: true,
			},
			kotsadmNamespace: "kotsadm",
			loading:          &spinner.MessageWriter{},
			validate: func(t *testing.T, opts kotscli.InstallOptions) {
				req := require.New(t)
				req.Equal("my-app", opts.AppSlug)
				req.Equal([]byte("license-data"), opts.License)
				req.Equal("kotsadm", opts.Namespace)
				req.Equal("test-cluster-id", opts.ClusterID)
				req.Equal("/path/to/bundle.airgap", opts.AirgapBundle)
				req.Equal("/path/to/config.yaml", opts.ConfigValuesFile)
				req.Equal(true, opts.SkipPreflights)
				req.NotEmpty(opts.ReplicatedAppEndpoint)
				req.NotNil(opts.Stdout)
			},
		},
		{
			name: "minimal options",
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug: "simple-app",
					},
				},
				licenseBytes: []byte("license-data"),
				clusterID:    "cluster-123",
			},
			flags: installFlags{
				airgapBundle:        "",
				configValues:        "",
				ignoreAppPreflights: false,
			},
			kotsadmNamespace: "default",
			loading:          nil,
			validate: func(t *testing.T, opts kotscli.InstallOptions) {
				req := require.New(t)
				req.Equal("simple-app", opts.AppSlug)
				req.Equal("default", opts.Namespace)
				req.Equal("", opts.AirgapBundle)
				req.Equal("", opts.ConfigValuesFile)
				req.Equal(false, opts.SkipPreflights)
				req.Nil(opts.Stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildKotsInstallOptions(tt.installCfg, tt.flags, tt.kotsadmNamespace, tt.loading)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}

func Test_buildAddonInstallOpts(t *testing.T) {
	// Set up release data with embedded cluster config for testing
	err := release.SetReleaseDataForTests(map[string][]byte{
		"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  roles:
    controller:
      name: controller-test
`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		release.SetReleaseDataForTests(nil)
	})

	tests := []struct {
		name             string
		flags            installFlags
		installCfg       *installConfig
		rc               runtimeconfig.RuntimeConfig
		kotsadmNamespace string
		loading          **spinner.MessageWriter
		validate         func(*testing.T, *addons.InstallOptions, runtimeconfig.RuntimeConfig, *installConfig)
	}{
		{
			name: "all features enabled",
			flags: installFlags{
				adminConsolePassword: "password123",
				airgapBundle:         "/path/to/bundle.airgap",
				hostname:             "example.com",
			},
			installCfg: &installConfig{
				clusterID: "cluster-123",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       true,
						IsEmbeddedClusterMultiNodeEnabled: true,
					},
				},
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(8800)
				rc.SetProxySpec(&ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8080",
				})
				rc.SetHostCABundlePath("/etc/ssl/certs/ca-bundle.crt")
				rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				})
				return rc
			}(t),
			kotsadmNamespace: "kotsadm",
			loading: func() **spinner.MessageWriter {
				loading := &spinner.MessageWriter{}
				return &loading
			}(),
			validate: func(t *testing.T, opts *addons.InstallOptions, rc runtimeconfig.RuntimeConfig, installCfg *installConfig) {
				req := require.New(t)
				req.Equal("cluster-123", opts.ClusterID)
				req.Equal("password123", opts.AdminConsolePwd)
				req.Equal(8800, opts.AdminConsolePort)
				req.Equal(true, opts.IsAirgap)
				req.Equal("example.com", opts.Hostname)
				req.Equal([]byte("cert-data"), opts.TLSCertBytes)
				req.Equal([]byte("key-data"), opts.TLSKeyBytes)
				req.Equal(true, opts.DisasterRecoveryEnabled)
				req.Equal(true, opts.IsMultiNodeEnabled)
				req.Equal("kotsadm", opts.KotsadmNamespace)
				req.Equal(rc.EmbeddedClusterHomeDirectory(), opts.DataDir)
				req.Equal(rc.EmbeddedClusterK0sSubDir(), opts.K0sDataDir)
				req.Equal(rc.EmbeddedClusterOpenEBSLocalSubDir(), opts.OpenEBSDataDir)
				req.Equal("10.96.0.0/12", opts.ServiceCIDR)
				req.Equal("/etc/ssl/certs/ca-bundle.crt", opts.HostCABundlePath)
				proxySpec := rc.ProxySpec()
				req.Equal(proxySpec, opts.ProxySpec)
				req.Equal(installCfg.license, opts.License)
				req.Equal(&installCfg.endUserConfig.Spec, opts.EndUserConfigSpec)
				expectedEmbeddedCfg := release.GetEmbeddedClusterConfig()
				req.NotNil(expectedEmbeddedCfg)
				req.Equal(&expectedEmbeddedCfg.Spec, opts.EmbeddedConfigSpec)
				req.NotNil(opts.KotsInstaller)
			},
		},
		{
			name: "minimal configuration",
			flags: installFlags{
				adminConsolePassword: "pass",
				airgapBundle:         "",
				hostname:             "",
			},
			installCfg: &installConfig{
				clusterID: "cluster-456",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       false,
						IsEmbeddedClusterMultiNodeEnabled: false,
					},
				},
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(30000)
				rc.SetHostCABundlePath("/etc/ssl/certs/ca-bundle.crt")
				rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				})
				return rc
			}(t),
			kotsadmNamespace: "kotsadm",
			loading: func() **spinner.MessageWriter {
				loading := &spinner.MessageWriter{}
				return &loading
			}(),
			validate: func(t *testing.T, opts *addons.InstallOptions, rc runtimeconfig.RuntimeConfig, installCfg *installConfig) {
				req := require.New(t)
				req.Equal("cluster-456", opts.ClusterID)
				req.Equal("pass", opts.AdminConsolePwd)
				req.Equal(30000, opts.AdminConsolePort)
				req.Equal(false, opts.IsAirgap)
				req.Equal("", opts.Hostname)
				req.Equal([]byte("cert-data"), opts.TLSCertBytes)
				req.Equal([]byte("key-data"), opts.TLSKeyBytes)
				req.Equal(false, opts.DisasterRecoveryEnabled)
				req.Equal(false, opts.IsMultiNodeEnabled)
				req.Equal("kotsadm", opts.KotsadmNamespace)
				req.Equal(rc.EmbeddedClusterHomeDirectory(), opts.DataDir)
				req.Equal(rc.EmbeddedClusterK0sSubDir(), opts.K0sDataDir)
				req.Equal(rc.EmbeddedClusterOpenEBSLocalSubDir(), opts.OpenEBSDataDir)
				req.Equal("10.96.0.0/12", opts.ServiceCIDR)
				req.Equal("/etc/ssl/certs/ca-bundle.crt", opts.HostCABundlePath)
				req.Nil(opts.ProxySpec)
				req.Equal(installCfg.license, opts.License)
				req.Nil(opts.EndUserConfigSpec)
				expectedEmbeddedCfg := release.GetEmbeddedClusterConfig()
				req.NotNil(expectedEmbeddedCfg)
				req.Equal(&expectedEmbeddedCfg.Spec, opts.EmbeddedConfigSpec)
				req.NotNil(opts.KotsInstaller)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildAddonInstallOpts(tt.flags, tt.installCfg, tt.rc, tt.kotsadmNamespace, tt.loading)

			if tt.validate != nil {
				tt.validate(t, opts, tt.rc, tt.installCfg)
			}
		})
	}
}

func Test_buildK0sConfig(t *testing.T) {
	tests := []struct {
		name       string
		flags      *installFlags
		installCfg *installConfig
		wantErr    bool
		validate   func(*testing.T, *k0sv1beta1.ClusterConfig)
	}{
		{
			name: "pod and service CIDRs set",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/24", cfg.Spec.Network.PodCIDR)
				req.Equal("10.1.0.0/24", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "custom pod and service CIDRs",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "192.168.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("192.168.0.0/16", cfg.Spec.Network.PodCIDR)
				req.Equal("10.96.0.0/12", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "global CIDR should not affect k0s config",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/25",
					ServiceCIDR: "10.0.0.128/25",
					GlobalCIDR:  stringPtr("10.0.0.0/24"),
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/25", cfg.Spec.Network.PodCIDR)
				req.Equal("10.0.0.128/25", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "IPv4 CIDRs with different masks",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "172.16.0.0/20",
					ServiceCIDR: "172.17.0.0/20",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("172.16.0.0/20", cfg.Spec.Network.PodCIDR)
				req.Equal("172.17.0.0/20", cfg.Spec.Network.ServiceCIDR)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			cfg, err := buildK0sConfig(tt.flags, tt.installCfg)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.NotNil(cfg)
			req.NotNil(cfg.Spec)
			req.NotNil(cfg.Spec.Network)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func Test_buildRecordInstallationOptions(t *testing.T) {
	// Set up release data with embedded cluster config for testing
	err := release.SetReleaseDataForTests(map[string][]byte{
		"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  version: "1.0.0"
  roles:
    controller:
      name: controller-test
`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		release.SetReleaseDataForTests(nil)
	})

	tests := []struct {
		name       string
		installCfg *installConfig
		rc         runtimeconfig.RuntimeConfig
		validate   func(*testing.T, kubeutils.RecordInstallationOptions)
	}{
		{
			name: "airgap with metadata and info",
			installCfg: &installConfig{
				clusterID: "cluster-123",
				isAirgap:  true,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: &kotsv1beta1.Airgap{
						Spec: kotsv1beta1.AirgapSpec{
							UncompressedSize: 1024 * 1024 * 1024,
						},
					},
				},
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(8800)
				rc.SetManagerPort(8801)
				return rc
			}(t),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal("cluster-123", opts.ClusterID)
				req.True(opts.IsAirgap)
				req.NotNil(opts.License)
				req.NotEmpty(opts.MetricsBaseURL)
				req.NotNil(opts.RuntimeConfig)
				req.NotEmpty(opts.RuntimeConfig.DataDir)
				req.Equal(8800, opts.RuntimeConfig.AdminConsole.Port)
				req.Equal(8801, opts.RuntimeConfig.Manager.Port)
				req.NotNil(opts.ConfigSpec)
				req.Equal("1.0.0", opts.ConfigSpec.Version)
				req.Equal("controller-test", opts.ConfigSpec.Roles.Controller.Name)
				req.Equal(int64(1024*1024*1024), opts.AirgapUncompressedSize)
				req.Nil(opts.EndUserConfig)
			},
		},
		{
			name: "airgap with metadata but no info",
			installCfg: &installConfig{
				clusterID: "cluster-456",
				isAirgap:  true,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: nil,
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal(int64(0), opts.AirgapUncompressedSize)
			},
		},
		{
			name: "non-airgap with end user config",
			installCfg: &installConfig{
				clusterID: "cluster-789",
				isAirgap:  false,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.False(opts.IsAirgap)
				req.NotNil(opts.EndUserConfig)
			},
		},
		{
			name: "minimal installation",
			installCfg: &installConfig{
				clusterID: "cluster-abc",
				isAirgap:  false,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal("cluster-abc", opts.ClusterID)
				req.False(opts.IsAirgap)
				req.Nil(opts.EndUserConfig)
				req.Equal(int64(0), opts.AirgapUncompressedSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildRecordInstallationOptions(tt.installCfg, tt.rc)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}
