package dryrun

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	oprelease "github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestV3Upgrade_HappyPathOnline(t *testing.T) {
	hcli := setupV3UpgradeTestHelmClient()
	licenseFile, configFile := setupV3UpgradeTest(t, hcli, nil)

	// Start upgrader in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"upgrade",
			"--target", "linux",
			"--license", licenseFile,
			"--yes",
		)
		if err != nil {
			t.Logf("upgrader exited with error: %v", err)
		}
	}()

	runV3Upgrade(t, v3UpgradeArgs{
		managerPort:         30080,
		password:            "password123",
		isAirgap:            false,
		configValuesFile:    configFile,
		ignoreAppPreflights: false,
	})

	validateHappyPathOnlineUpgrade(t, hcli)
}

func validateHappyPathOnlineUpgrade(t *testing.T, hcli *helm.MockClient) {
	adminConsoleNamespace := "fake-app-slug"

	// Load dryrun output to validate registry resources are NOT created
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.NotEmpty(t, in.Spec.ClusterID, "Installation.Spec.ClusterID should be set")
	assert.False(t, in.Spec.AirGap, "Installation.Spec.AirGap should be false for online installations")
	assert.Equal(t, int64(0), in.Spec.AirgapUncompressedSize, "Installation.Spec.AirgapUncompressedSize should be 0 for online installations")
	assert.Equal(t, "80-32767", in.Spec.RuntimeConfig.Network.NodePortRange, "Installation.Spec.RuntimeConfig.Network.NodePortRange should be set to default range")

	// Validate addons

	// Validate embedded-cluster-operator addon
	operatorOpts, found := isHelmReleaseUpgraded(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be upgraded")
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"embeddedClusterID": in.Spec.ClusterID,
	})
	// Validate that isAirgap helm value is not set in embedded-cluster-operator chart for online installations
	_, hasIsAirgap := operatorOpts.Values["isAirgap"]
	assert.False(t, hasIsAirgap, "embedded-cluster-operator should not have isAirgap helm value for online installations")

	// Validate admin-console addon
	adminConsoleOpts, found := isHelmReleaseUpgraded(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be upgraded")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"isAirgap":           false,
		"isMultiNodeEnabled": true,
		"embeddedClusterID":  in.Spec.ClusterID,
	})

	// Validate that registry addon is NOT upgraded for online installations
	_, found = isHelmReleaseUpgraded(hcli, "docker-registry")
	require.False(t, found, "docker-registry helm release should not be upgraded")

	// Validate that registry-creds secret is NOT created for online installations
	assertSecretNotExists(t, kcli, "registry-creds", adminConsoleNamespace)

	// Validate OS environment variables use default data directory
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/fake-app-slug/tmp",
		"KUBECONFIG": "/var/lib/fake-app-slug/k0s/pki/admin.conf",
	})

	// Validate KOTS CLI deploy
	assertCommands(t, dr.Commands,
		[]any{
			regexp.MustCompile(`kubectl-kots.* deploy fake-app-slug --config-values .* --channel-id fake-channel-id --channel-sequence 2 --namespace fake-app-slug --skip-preflights`),
		},
		false,
	)
}

func TestV3Upgrade_HappyPathAirgap(t *testing.T) {
	hcli := setupV3UpgradeTestHelmClient()
	licenseFile, configFile := setupV3UpgradeTest(t, hcli, &v3UpgradeSetupArgs{
		isAirgap: true,
	})

	airgapPath := upgradeAirgapBundleFile(t)

	// Start upgrader in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"upgrade",
			"--target", "linux",
			"--license", licenseFile,
			"--airgap-bundle", airgapPath,
			"--yes",
		)
		if err != nil {
			t.Logf("upgrader exited with error: %v", err)
		}
	}()

	runV3Upgrade(t, v3UpgradeArgs{
		managerPort:         30080,
		password:            "password123",
		isAirgap:            true,
		configValuesFile:    configFile,
		ignoreAppPreflights: false,
	})

	validateHappyPathAirgapUpgrade(t, hcli)
}

func validateHappyPathAirgapUpgrade(t *testing.T, hcli *helm.MockClient) {
	adminConsoleNamespace := "fake-app-slug"

	// Load dryrun output to validate registry resources ARE created
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object has correct AirGap settings
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.True(t, in.Spec.AirGap, "Installation.Spec.AirGap should be true for airgap installations")
	// TODO: fix this test
	// assert.Greater(t, in.Spec.AirgapUncompressedSize, int64(0), "Installation.Spec.AirgapUncompressedSize should be greater than 0 for airgap installations")

	// Validate that registry addon IS upgraded for airgap installations
	_, found := isHelmReleaseUpgraded(hcli, "docker-registry")
	require.True(t, found, "docker-registry helm release should be upgraded")

	// Validate that isAirgap helm value is set to true in admin console chart
	adminConsoleOpts, found := isHelmReleaseUpgraded(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be upgraded")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"isAirgap": true,
	})

	// Validate that isAirgap helm value is set to "true" in embedded-cluster-operator chart for airgap installations
	operatorOpts, found := isHelmReleaseUpgraded(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be upgraded")
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"isAirgap": "true",
	})

	// Validate that registry-creds secret IS created for airgap installations
	assertSecretExists(t, kcli, "registry-creds", adminConsoleNamespace)

	// Validate KOTS CLI deploy command includes --airgap-bundle flag for airgap installations
	assertCommands(t, dr.Commands,
		[]any{
			regexp.MustCompile(`kubectl-kots.* deploy fake-app-slug --config-values .* --license .* --airgap-bundle .* --disable-image-push --namespace fake-app-slug --skip-preflights`),
		},
		false,
	)
}

// v3UpgradeArgs are the configurable request arguments for the reusable non-headless upgrade flow
type v3UpgradeArgs struct {
	managerPort         int
	password            string
	isAirgap            bool
	configValuesFile    string
	ignoreAppPreflights bool
}

// runV3Upgrade executes the non-headless upgrade user flow against the API using the provided arguments.
func runV3Upgrade(t *testing.T, args v3UpgradeArgs) {
	t.Helper()

	ctx := t.Context()

	// Wait for API be ready
	httpClient := insecureHTTPClient()
	waitForAPIReady(t, httpClient, fmt.Sprintf("https://localhost:%d/api/health", args.managerPort))

	// Build API client and authenticate
	c := apiclient.New(fmt.Sprintf("https://localhost:%d", args.managerPort), apiclient.WithHTTPClient(httpClient))
	require.NoError(t, c.Authenticate(ctx, args.password))

	// Configure application with config values
	kcv, err := helpers.ParseConfigValues(args.configValuesFile)
	require.NoError(t, err, "failed to parse config values file")
	appConfigValues := apitypes.ConvertToAppConfigValues(kcv)
	_, err = c.PatchLinuxUpgradeAppConfigValues(ctx, appConfigValues)
	require.NoError(t, err)

	// If airgap, process airgap and wait for completion
	if args.isAirgap {
		_, err = c.ProcessLinuxUpgradeAirgap(ctx)
		require.NoError(t, err)
		assertEventuallySucceeded(t, "airgap processing", func() (apitypes.State, string, error) {
			st, err := c.GetLinuxUpgradeAirgapStatus(ctx)
			if err != nil {
				return "", "", err
			}
			return st.Status.State, st.Status.Description, nil
		})
	}

	// Run infrastructure upgrade and wait for completion
	_, err = c.UpgradeLinuxInfra(ctx)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "infrastructure upgrade", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxUpgradeInfraStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Run app preflights and wait for completion
	_, err = c.RunLinuxUpgradeAppPreflights(ctx)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "application preflights", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxUpgradeAppPreflightsStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Upgrade application and wait for completion
	_, err = c.UpgradeLinuxApp(ctx, args.ignoreAppPreflights)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "application upgrade", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxAppUpgradeStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Dump and load dryrun output for inspection/assertions
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")
}

type v3UpgradeSetupArgs struct {
	currentECVersion        string
	dataDir                 string
	networkSpec             *ecv1beta1.NetworkSpec
	proxySpec               *ecv1beta1.ProxySpec
	adminConsoleSpec        *ecv1beta1.AdminConsoleSpec
	localArtifactMirrorSpec *ecv1beta1.LocalArtifactMirrorSpec
	managerSpec             *ecv1beta1.ManagerSpec
	isAirgap                bool
}

func setupV3UpgradeTest(t *testing.T, hcli helm.Client, setupArgs *v3UpgradeSetupArgs) (string, string) {
	// Set ENABLE_V3 environment variable
	t.Setenv("ENABLE_V3", "1")

	// Ensure UI assets are available when starting API in non-headless tests
	prepareWebAssetsForTests(t)

	// Setup release data for tests
	if err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml":        []byte(upgradeReleaseData),
		"cluster-config.yaml": []byte(upgradeClusterConfigData),
		"application.yaml":    []byte(applicationData),
		"config.yaml":         []byte(configData),
		"chart.yaml":          []byte(helmChartData),
		"nginx-app-0.1.0.tgz": []byte(nginxChartArchiveData),
		"redis-app-0.1.0.tgz": []byte(redisChartArchiveData),
	}); err != nil {
		t.Fatalf("fail to set release data: %v", err)
	}

	rel := release.GetReleaseData()
	require.NotNil(t, rel)

	// Ensure a valid Local Artifact Mirror image reference for distribute artifacts flow
	if versions.LocalArtifactMirrorImage == "" {
		versions.LocalArtifactMirrorImage = "proxy.replicated.com/anonymous/replicated/local-artifact-mirror:v2.12.0-k8s-1.33-amd64"
	}

	// Initialize dryrun with mocks
	hlprs := &dryrun.Helpers{}
	hlprs.SetCommandStubs([]dryrun.CommandStub{
		{
			Pattern: regexp.MustCompile(`kubectl-kots.* get config .*`),
			Respond: func(args []string) (string, error) {
				return `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    text_required:
      value: "initial text required value"
    text_required_with_regex:
      value: "salah@replicated.com"
    password_required:
      value: "initial password required value"
    file_required:
      value: "ZmlsZSByZXF1aXJlZCB2YWx1ZQo="
      filename: "file_required.txt"`, nil
			},
		},
		{
			Pattern: regexp.MustCompile(`kubectl-kots.* get versions .* -o json`),
			Respond: func(args []string) (string, error) {
				return `[{"versionLabel":"current-version","channelSequence":1,"sequence":1,"status":"deployed"}]`, nil
			},
		},
	})

	if hcli == nil {
		hcli = setupV3UpgradeTestHelmClient()
	}
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	k0sClient := &dryrun.K0s{}
	dryrun.Init(drFile, &dryrun.Client{
		K0sClient:  k0sClient,
		HelmClient: hcli,
		Helpers:    hlprs,
		ReplicatedAPIClient: &dryrun.ReplicatedAPIClient{
			LicenseBytes: []byte(licenseData),
		},
	})
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetOutput(os.Stdout)

	// Set k0s status so runtimeconfig can be discovered from cluster
	k0sClient.Status = &k0s.K0sStatus{
		Vars: k0s.K0sVars{
			AdminKubeConfigPath: fmt.Sprintf("/var/lib/%s/k0s/pki/admin.conf", rel.ChannelRelease.AppSlug),
		},
	}

	// Seed existing cluster state required by upgrade pre-run
	kcli, err := dryrun.KubeClient()
	require.NoError(t, err)
	seedClusterDataForUpgrade(t, kcli, setupArgs)

	// Create license file
	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Create config values file
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	createConfigValuesFile(t, configFile)

	return licenseFile, configFile
}

func setupV3UpgradeTestHelmClient() *helm.MockClient {
	hcli := &helm.MockClient{}
	// Upgrades for addons (namespace and name arguments are not asserted here)
	hcli.On("ReleaseExists", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	hcli.On("Upgrade", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	hcli.
		On("Render", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "nginx-app"
		})).
		Return([][]byte{[]byte(renderedChartPreflightData)}, nil).
		Maybe()
	hcli.On("Close").Return(nil).Maybe()
	return hcli
}

func isHelmReleaseUpgraded(hcli *helm.MockClient, releaseName string) (helm.UpgradeOptions, bool) {
	for _, call := range hcli.Calls {
		if call.Method == "Upgrade" {
			opts := call.Arguments[1].(helm.UpgradeOptions)
			if opts.ReleaseName == releaseName {
				return opts, true
			}
		}
	}
	return helm.UpgradeOptions{}, false
}

func seedClusterDataForUpgrade(t *testing.T, kcli ctrlclient.Client, setupArgs *v3UpgradeSetupArgs) {
	t.Helper()

	if setupArgs == nil {
		setupArgs = &v3UpgradeSetupArgs{}
	}

	rel := release.GetReleaseData()
	require.NotNil(t, rel)
	cfgSpec := rel.EmbeddedClusterConfig.Spec

	// Seed a minimal k0s ClusterConfig CR
	k0sCfg := &k0sv1beta1.ClusterConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterConfig",
			APIVersion: "k0s.k0sproject.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k0s",
			Namespace: "kube-system",
		},
	}
	require.NoError(t, kcli.Create(t.Context(), k0sCfg))

	// Create secrets required by upgrade
	ns := rel.ChannelRelease.AppSlug // new installs use app slug as namespace

	// tls secret with cert/key from assets
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-tls",
			Namespace: ns,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": testCertPEM,
			"tls.key": testKeyPEM,
		},
	}
	require.NoError(t, kcli.Create(t.Context(), tlsSecret))

	// password secret with bcrypt of "password123"
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	pwdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-password",
			Namespace: ns,
		},
		Data: map[string][]byte{
			"passwordBcrypt": hash,
		},
	}
	require.NoError(t, kcli.Create(t.Context(), pwdSecret))

	// seed release metadata
	releaseMeta := ectypes.ReleaseMetadata{
		Images: []string{
			"proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image:v2.12.0-k8s-1.33-amd64",
			"proxy.replicated.com/anonymous/replicated/embedded-cluster-utils:latest-amd64",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "oci://proxy.replicated.com/library/embedded-cluster-operator",
					Version:   cfgSpec.Version,
				},
			},
		},
	}

	if !setupArgs.isAirgap {
		// cache release metadata to avoid any replicated.app calls in online installations
		oprelease.CacheMeta(cfgSpec.Version, releaseMeta)
	} else {
		// create local release metadata configmap to avoid any registry pulls in airgap installations
		metaJSON, err := json.Marshal(releaseMeta)
		require.NoError(t, err)
		nsn := oprelease.LocalVersionMetadataConfigmap(cfgSpec.Version)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nsn.Name,
				Namespace: nsn.Namespace,
			},
			Data: map[string]string{
				"metadata.json": string(metaJSON),
			},
		}
		require.NoError(t, kcli.Create(t.Context(), cm))

		// Seed registry-creds secret
		dockerCfgJSON := []byte(`{"auths":{"127.0.0.1:5000":{"username":"embedded-cluster","password":"password"}}}`)
		regCreds := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-creds",
				Namespace: ns,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": dockerCfgJSON,
			},
		}
		require.NoError(t, kcli.Create(t.Context(), regCreds))
	}

	// Seed installation custom resource

	currCfgSpec := cfgSpec
	if setupArgs.currentECVersion != "" {
		currCfgSpec.Version = setupArgs.currentECVersion
	} else {
		currCfgSpec.Version = "2.11.0+k8s-1.32" // release file has 2.12.0+k8s-1.33
	}

	// setup runtime config
	rc := runtimeconfig.New(nil)

	if setupArgs.dataDir != "" {
		rc.SetDataDir(setupArgs.dataDir)
	} else {
		rc.SetDataDir(fmt.Sprintf("/var/lib/%s", rel.ChannelRelease.AppSlug))
	}

	if setupArgs.networkSpec != nil {
		rc.SetNetworkSpec(*setupArgs.networkSpec)
	} else {
		podCIDR, serviceCIDR, err := newconfig.SplitCIDR(ecv1beta1.DefaultNetworkCIDR)
		require.NoError(t, err)
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
			PodCIDR:          podCIDR,
			ServiceCIDR:      serviceCIDR,
			NodePortRange:    ecv1beta1.DefaultNetworkNodePortRange,
		})
	}

	if setupArgs.proxySpec != nil {
		rc.SetProxySpec(setupArgs.proxySpec)
	}
	if setupArgs.adminConsoleSpec != nil {
		rc.SetAdminConsolePort(setupArgs.adminConsoleSpec.Port)
	}
	if setupArgs.localArtifactMirrorSpec != nil {
		rc.SetLocalArtifactMirrorPort(setupArgs.localArtifactMirrorSpec.Port)
	}
	if setupArgs.managerSpec != nil {
		rc.SetManagerPort(setupArgs.managerSpec.Port)
	}

	installation := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			ClusterID:     "upgrade-cluster-id-123",
			Config:        &currCfgSpec,
			RuntimeConfig: rc.Get(),
		},
	}
	if setupArgs.isAirgap {
		installation.Spec.AirGap = true
		installation.Spec.Artifacts = &ecv1beta1.ArtifactsLocation{
			Images:                  "images",
			HelmCharts:              "helm-charts",
			EmbeddedClusterBinary:   "embedded-cluster-binary",
			EmbeddedClusterMetadata: "embedded-cluster-metadata",
		}
	}
	require.NoError(t, kcli.Create(t.Context(), installation))
}

var (
	//go:embed assets/upgrade-bundle.airgap
	upgradeAirgapBundle []byte
)

func upgradeAirgapBundleFile(t *testing.T) string {
	bundleAirgapFile := filepath.Join(t.TempDir(), "upgrade-bundle.airgap")
	require.NoError(t, os.WriteFile(bundleAirgapFile, upgradeAirgapBundle, 0644))
	return bundleAirgapFile
}
