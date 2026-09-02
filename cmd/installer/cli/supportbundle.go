package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// kotsadmNamespace is the namespace where the KOTS admin console and its redactor
	// secrets are deployed. This is currently hardcoded because
	// runtimeconfig.KotsadmNamespace requires a controller-runtime client.Client, while
	// this code uses a typed kubernetes.Interface clientset.
	kotsadmNamespace = "kotsadm"
)

func SupportBundleCmd(ctx context.Context) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "support-bundle",
		Short: "Generate a support bundle",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}

			rc = rcutil.InitBestRuntimeConfig(cmd.Context())
			return rc.SetEnv()
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			supportBundle := rc.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				return errors.New("support-bundle command can only be run after an install attempt")
			}

			hostSupportBundle := rc.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
			if _, err := os.Stat(hostSupportBundle); err != nil {
				return fmt.Errorf("unable to find host support bundle: %w", err)
			}

			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get current working directory: %w", err)
			}
			now := time.Now().Format("2006-01-02T15_04_05")
			fname := fmt.Sprintf("support-bundle-%s.tar.gz", now)
			destination := filepath.Join(pwd, fname)

			kubeConfig := rc.PathToKubeConfig()
			arguments := []string{}
			if _, err := os.Stat(kubeConfig); err == nil {
				arguments = append(arguments, fmt.Sprintf("--kubeconfig=%s", kubeConfig))
			}

			clientset, err := kubeutils.GetClientset()
			if err != nil {
				logrus.Warnf("Unable to create kubernetes client, redactor specs will not be applied: %v", err)
			} else {
				redactorURIs := buildRedactorURIs(cmd.Context(), runtimeconfig.AppSlug(), clientset)
				if len(redactorURIs) > 0 {
					arguments = append(arguments, fmt.Sprintf("--redactors=%s", strings.Join(redactorURIs, ",")))
				}
			}

			arguments = append(
				arguments,
				"--interactive=false",
				"--load-cluster-specs",
				fmt.Sprintf("--output=%s", destination),
				hostSupportBundle,
			)

			spin := spinner.Start()
			spin.Infof("Generating support bundle (this can take a while)")

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{
					Stdout:       stdout,
					Stderr:       stderr,
					Env:          map[string]string{"TROUBLESHOOT_AUTO_UPDATE": "false"},
					LogOnSuccess: true,
				},
				supportBundle,
				arguments...,
			); err != nil {
				spin.ErrorClosef("Failed to generate support bundle")
				io.Copy(os.Stdout, stdout)
				io.Copy(os.Stderr, stderr)
				return NewErrorNothingElseToAdd(errors.New("failed to generate support bundle"))
			}

			spin.Closef("Support bundle saved at %s", destination)
			return nil
		},
	}

	return cmd
}

// buildRedactorURIs returns the list of KOTS redactor URIs that should be passed to
// kubectl-support_bundle. It checks for the existence of each redactor secret in the
// cluster and only includes URIs for secrets that exist. A warning is logged when the
// app-specific redactor secret is missing because that means vendor-defined redactors
// will not be applied.
func buildRedactorURIs(ctx context.Context, appSlug string, clientset kubernetes.Interface) []string {
	uris := []string{}

	if secretExists(ctx, kotsadmNamespace, "kotsadm-redact-spec", clientset) {
		uris = append(uris, fmt.Sprintf("secret/%s/kotsadm-redact-spec/redact-spec", kotsadmNamespace))
	}

	appRedactorSecretName := fmt.Sprintf("kotsadm-%s-redact-spec", appSlug)
	if secretExists(ctx, kotsadmNamespace, appRedactorSecretName, clientset) {
		uris = append(uris, fmt.Sprintf("secret/%s/%s/redact-spec", kotsadmNamespace, appRedactorSecretName))
	} else {
		logrus.Warnf("App-specific redactor secret %q not found in namespace %q; support bundle may contain unredacted application data", appRedactorSecretName, kotsadmNamespace)
	}

	if secretExists(ctx, kotsadmNamespace, "kotsadm-redact-default-spec", clientset) {
		uris = append(uris, fmt.Sprintf("secret/%s/kotsadm-redact-default-spec/default-redactor", kotsadmNamespace))
	}

	return uris
}

func secretExists(ctx context.Context, namespace, name string, clientset kubernetes.Interface) bool {
	_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logrus.Debugf("Failed to check for secret %s/%s: %v", namespace, name, err)
		}
		return false
	}
	return true
}
