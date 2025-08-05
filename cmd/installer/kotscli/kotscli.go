package kotscli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
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

	licenseFile, err := os.CreateTemp("", "license")
	if err != nil {
		return fmt.Errorf("unable to create temp file: %w", err)
	}
	defer os.Remove(licenseFile.Name())

	if _, err := licenseFile.Write(opts.License); err != nil {
		return fmt.Errorf("unable to write license to temp file: %w", err)
	}

	maskfn := MaskKotsOutputForOnline()
	installArgs := []string{
		"install",
		upstreamURI,
		"--license-file",
		licenseFile.Name(),
		"--namespace",
		opts.Namespace,
		"--app-version-label",
		appVersionLabel,
		"--exclude-admin-console",
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
