package kotscli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
)

var (
	CounterRegex = regexp.MustCompile(`(\d+)/(\d+)`)
)

type InstallOptions struct {
	AppSlug               string
	License               []byte
	Namespace             string
	ClusterID             string
	AirgapBundle          string
	ConfigValuesFile      string
	ReplicatedAppEndpoint string
	SkipPreflights        bool
	DisableImagePush      bool
	Stdout                io.Writer
}

func Install(opts InstallOptions) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	var appVersionLabel string
	var channelSlug string
	if channelRelease := release.GetChannelRelease(); channelRelease != nil {
		appVersionLabel = channelRelease.VersionLabel
		channelSlug = channelRelease.ChannelSlug
	}

	upstreamURI := opts.AppSlug
	if channelSlug != "" && channelSlug != "stable" {
		upstreamURI = fmt.Sprintf("%s/%s", upstreamURI, channelSlug)
	}

	licenseFile, err := createLicenseFile(opts.License)
	if err != nil {
		return fmt.Errorf("create temp license file: %w", err)
	}
	defer os.Remove(licenseFile)

	maskfn := MaskKotsOutputForOnline()
	installArgs := []string{
		"install",
		upstreamURI,
		"--license-file",
		licenseFile,
		"--namespace",
		opts.Namespace,
		"--app-version-label",
		appVersionLabel,
		"--exclude-admin-console",
	}
	if opts.DisableImagePush {
		installArgs = append(installArgs, "--disable-image-push")
	}
	if opts.AirgapBundle != "" {
		installArgs = append(installArgs, "--airgap-bundle", opts.AirgapBundle)
		maskfn = MaskKotsOutputForAirgap()
	}

	if opts.ConfigValuesFile != "" {
		installArgs = append(installArgs, "--config-values", opts.ConfigValuesFile)
	}

	if opts.SkipPreflights {
		installArgs = append(installArgs, "--skip-preflights")
	}

	if msg, ok := opts.Stdout.(*spinner.MessageWriter); ok && msg != nil {
		msg.SetMask(maskfn)
		defer msg.SetMask(nil)
	}

	runCommandOptions := helpers.RunCommandOptions{
		LogOnSuccess: true,
		Env: map[string]string{
			"EMBEDDED_CLUSTER_ID": opts.ClusterID,
			"POD_NAMESPACE":       opts.Namespace, // This is required for kots to find the registry-creds secret
		},
	}
	if opts.Stdout != nil {
		runCommandOptions.Stdout = opts.Stdout
	}
	if opts.ReplicatedAppEndpoint != "" {
		runCommandOptions.Env["REPLICATED_APP_ENDPOINT"] = opts.ReplicatedAppEndpoint
	}
	err = helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, installArgs...)
	if err != nil {
		return fmt.Errorf("unable to install the application: %w", err)
	}

	return nil
}

func ResetPassword(rc runtimeconfig.RuntimeConfig, password string) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	runCommandOptions := helpers.RunCommandOptions{
		Env:   map[string]string{"KUBECONFIG": rc.PathToKubeConfig()},
		Stdin: strings.NewReader(fmt.Sprintf("%s\n", password)),
	}

	resetArgs := []string{"reset-password", "kotsadm"}
	if err := helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, resetArgs...); err != nil {
		return fmt.Errorf("unable to reset admin console password: %w", err)
	}

	return nil
}

type AirgapUpdateOptions struct {
	AppSlug      string
	Namespace    string
	AirgapBundle string
	ClusterID    string
}

func AirgapUpdate(opts AirgapUpdateOptions) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	maskfn := MaskKotsOutputForAirgap()

	airgapUpdateArgs := []string{
		"airgap-update",
		opts.AppSlug,
		"--namespace",
		opts.Namespace,
		"--airgap-bundle",
		opts.AirgapBundle,
	}

	logrus.Info("")
	loading := spinner.Start(spinner.WithMask(maskfn))
	runCommandOptions := helpers.RunCommandOptions{
		Stdout: loading,
		Env: map[string]string{
			"EMBEDDED_CLUSTER_ID": opts.ClusterID,
		},
	}
	if err := helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, airgapUpdateArgs...); err != nil {
		loading.ErrorClosef("Failed to update")
		return fmt.Errorf("unable to update the application: %w", err)
	}

	loading.Closef("Update complete")

	logrus.Info("\n\033[1m" +
		"----------------------------------------------\n" +
		"Visit the Admin Console to deploy this update.\n" +
		"----------------------------------------------" +
		"\033[0m\n")
	return nil
}

type VeleroConfigureOtherS3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	Path            string
	AccessKeyID     string
	SecretAccessKey string
	Namespace       string
}

func VeleroConfigureOtherS3(opts VeleroConfigureOtherS3Options) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	veleroConfigureOtherS3Args := []string{
		"velero",
		"configure-other-s3",
		"--endpoint",
		opts.Endpoint,
		"--region",
		opts.Region,
		"--bucket",
		opts.Bucket,
		"--access-key-id",
		opts.AccessKeyID,
		"--secret-access-key",
		opts.SecretAccessKey,
		"--namespace",
		opts.Namespace,
		"--skip-validation",
	}
	if opts.Path != "" {
		veleroConfigureOtherS3Args = append(veleroConfigureOtherS3Args, "--path", opts.Path)
	}

	loading := spinner.Start()
	loading.Infof("Configuring backup storage location")

	if _, err := helpers.RunCommand(kotsBinPath, veleroConfigureOtherS3Args...); err != nil {
		loading.ErrorClosef("Failed to configure backup storage location")
		return fmt.Errorf("unable to configure s3: %w", err)
	}

	loading.Closef("Backup storage location configured")
	return nil
}

// MaskKotsOutputForOnline masks the kots cli output during online installations. For
// online installations we only want to print "Finalizing Admin Console" until it is done
// and then print "Finished".
func MaskKotsOutputForOnline() spinner.MaskFn {
	return func(message string) string {
		if strings.Contains(message, "Finished") {
			return message
		}
		return "Finalizing Admin Console"
	}
}

// MaskKotsOutputForAirgap masks the kots cli output during airgap installations. This
// function replaces some of the messages being printed to the user so the output looks
// nicer.
func MaskKotsOutputForAirgap() spinner.MaskFn {
	current := "Extracting air gap bundle"
	return func(message string) string {
		switch {
		case strings.Contains(message, "Pushing application images"):
			current = message
		case strings.Contains(message, "Pushing embedded cluster artifacts"):
			current = strings.ReplaceAll(message, "embedded cluster", "additional")
		case strings.Contains(message, "Waiting for the Admin Console"):
			current = "Finalizing Admin Console"
		case strings.Contains(message, "Update complete"):
			current = message
		}
		return current
	}
}

func GetJoinCommand(ctx context.Context, rc runtimeconfig.RuntimeConfig) (string, error) {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return "", fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	outBuffer := bytes.NewBuffer(nil)
	runCommandOptions := helpers.RunCommandOptions{
		Context: ctx,
		Env:     map[string]string{"KUBECONFIG": rc.PathToKubeConfig()},
		Stdin:   strings.NewReader(""),
		Stdout:  outBuffer,
	}

	resetArgs := []string{"get", "join-command", "-n", "kotsadm"}
	if err := helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, resetArgs...); err != nil {
		return "", fmt.Errorf("unable to get join command: %w", err)
	}

	return outBuffer.String(), nil
}

// DeployOptions represents options for deploying an application using the new kots deploy command
type DeployOptions struct {
	AppSlug               string
	License               []byte
	Namespace             string
	ClusterID             string
	AirgapBundle          string
	ConfigValuesFile      string
	ChannelID             string
	ChannelSequence       int64
	ReplicatedAppEndpoint string
	SkipPreflights        bool
	Stdout                io.Writer
}

// Deploy performs an application deployment using the new KOTS deploy command
// This combines license sync, upstream update download, configuration, and deployment in a single atomic operation
func Deploy(opts DeployOptions) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	deployArgs := []string{
		"deploy",
		opts.AppSlug,
		"--config-values",
		opts.ConfigValuesFile,
	}

	if opts.AirgapBundle != "" {
		// Airgap deployment - add license file and airgap bundle
		if len(opts.License) == 0 {
			return fmt.Errorf("license is required for airgap deployments")
		}

		licenseFile, err := createLicenseFile(opts.License)
		if err != nil {
			return fmt.Errorf("create temp license file: %w", err)
		}
		defer os.Remove(licenseFile)

		deployArgs = append(deployArgs, "--license", licenseFile)
		deployArgs = append(deployArgs, "--airgap-bundle", opts.AirgapBundle)
		// Disable image push since we handle it separately earlier in the install / upgrade process
		deployArgs = append(deployArgs, "--disable-image-push")
	} else {
		// Online deployment - add channel info
		if opts.ChannelID == "" {
			return fmt.Errorf("channel id is required for online deployments")
		}
		if opts.ChannelSequence == 0 {
			return fmt.Errorf("channel sequence is required for online deployments")
		}

		deployArgs = append(deployArgs, "--channel-id", opts.ChannelID)
		deployArgs = append(deployArgs, "--channel-sequence", strconv.FormatInt(opts.ChannelSequence, 10))
	}

	if opts.Namespace != "" {
		deployArgs = append(deployArgs, "--namespace", opts.Namespace)
	}

	if opts.SkipPreflights {
		deployArgs = append(deployArgs, "--skip-preflights")
	}

	runCommandOptions := helpers.RunCommandOptions{
		LogOnSuccess: true,
		Env: map[string]string{
			"EMBEDDED_CLUSTER_ID": opts.ClusterID,
		},
	}
	if opts.Stdout != nil {
		runCommandOptions.Stdout = opts.Stdout
	}
	if opts.ReplicatedAppEndpoint != "" {
		runCommandOptions.Env["REPLICATED_APP_ENDPOINT"] = opts.ReplicatedAppEndpoint
	}

	err = helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, deployArgs...)
	if err != nil {
		return fmt.Errorf("run deploy command: %w", err)
	}

	return nil
}

// PushImagesOptions represents options for pushing images to a registry
type PushImagesOptions struct {
	AirgapBundle     string
	RegistryAddress  string
	RegistryUsername string
	RegistryPassword string
	ClusterID        string
	Stdout           io.Writer
}

// PushImages pushes application images to a registry using kots admin-console push-images
func PushImages(opts PushImagesOptions) error {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	pushArgs := []string{
		"admin-console",
		"push-images",
		opts.AirgapBundle,
		opts.RegistryAddress,
		"--registry-username",
		opts.RegistryUsername,
		"--registry-password",
		opts.RegistryPassword,
	}

	runCommandOptions := helpers.RunCommandOptions{
		LogOnSuccess: true,
		Env: map[string]string{
			"EMBEDDED_CLUSTER_ID": opts.ClusterID,
		},
	}
	if opts.Stdout != nil {
		runCommandOptions.Stdout = opts.Stdout
	}

	err = helpers.RunCommandWithOptions(runCommandOptions, kotsBinPath, pushArgs...)
	if err != nil {
		return fmt.Errorf("push images: %w", err)
	}

	return nil
}

func createLicenseFile(license []byte) (string, error) {
	licenseFile, err := os.CreateTemp("", "license")
	if err != nil {
		return "", fmt.Errorf("create temp license file: %w", err)
	}
	defer licenseFile.Close()

	if _, err := licenseFile.Write(license); err != nil {
		_ = os.Remove(licenseFile.Name())
		return "", fmt.Errorf("write license to temp file: %w", err)
	}

	return licenseFile.Name(), nil
}

// AppVersionInfo holds information about a deployed app version
type AppVersionInfo struct {
	VersionLabel    string `json:"versionLabel"`
	ChannelSequence int64  `json:"channelSequence"`
	Sequence        int64  `json:"sequence"`
	Status          string `json:"status"`
}

// GetCurrentAppVersion retrieves the currently deployed app version and sequence
func GetCurrentAppVersion(appSlug string, namespace string) (*AppVersionInfo, error) {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return nil, fmt.Errorf("materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	// Build command arguments: kots get versions <appSlug> -n <namespace> -o json
	args := []string{
		"get", "versions",
		appSlug,
		"-n", namespace,
		"-o", "json",
	}

	// Execute the command and capture output
	var outputBuffer bytes.Buffer
	runCommandOpts := helpers.RunCommandOptions{
		Stdout: &outputBuffer,
	}

	if err := helpers.RunCommandWithOptions(runCommandOpts, kotsBinPath, args...); err != nil {
		return nil, fmt.Errorf("get versions from kots: %w", err)
	}

	// Parse JSON output
	var versions []AppVersionInfo
	if err := json.Unmarshal(outputBuffer.Bytes(), &versions); err != nil {
		return nil, fmt.Errorf("unmarshal versions output: %w", err)
	}

	version, err := getLastDeployedAppVersion(versions)
	if err != nil {
		return nil, fmt.Errorf("no deployed version found for app %s", appSlug)
	}
	return version, nil
}

// getLastDeployedAppVersion finds the last deployed version from a slice of versions
func getLastDeployedAppVersion(versions []AppVersionInfo) (*AppVersionInfo, error) {
	// Find the last deployed version. This can be either successful or failed deploys.
	for _, v := range versions {
		if v.Status == "deployed" || v.Status == "failed" {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("no deployed version found")
}

// GetConfigValuesOptions holds options for getting config values
type GetConfigValuesOptions struct {
	AppSlug               string
	Namespace             string
	ClusterID             string
	ReplicatedAppEndpoint string
}

// GetConfigValues executes the kots get config command and returns the YAML output
func GetConfigValues(opts GetConfigValuesOptions) (string, error) {
	kotsBinPath, err := goods.InternalBinary("kubectl-kots")
	if err != nil {
		return "", fmt.Errorf("materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	// Build command arguments
	args := []string{
		"get", "config",
		"--appslug", opts.AppSlug,
		"--namespace", opts.Namespace,
		"--current",
		"--decrypt",
	}

	// Execute the command and capture output
	var outputBuffer strings.Builder
	runCommandOpts := helpers.RunCommandOptions{
		Stdout: &outputBuffer,
		Env: map[string]string{
			"EMBEDDED_CLUSTER_ID": opts.ClusterID,
		},
	}
	if opts.ReplicatedAppEndpoint != "" {
		runCommandOpts.Env["REPLICATED_APP_ENDPOINT"] = opts.ReplicatedAppEndpoint
	}

	if err := helpers.RunCommandWithOptions(runCommandOpts, kotsBinPath, args...); err != nil {
		return "", fmt.Errorf("get current config values from kots: %w", err)
	}

	return outputBuffer.String(), nil
}
