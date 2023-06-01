package cli //nolint:dupl

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/k0sproject/k0s/pkg/certificate"
	k0sconfig "github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/spf13/cobra"

	"github.com/emosbaugh/helmbin/pkg/config"
)

// NewCmdKubeconfig returns a cobra command for getting the kubeconfig
// This is pretty much a copy of the k0s kubeconfig command
// https://github.com/k0sproject/k0s/tree/v1.26.3%2Bk0s.1/cmd/kubeconfig
func NewCmdKubeconfig(_ *CLI) *cobra.Command {
	opts := &config.CLIOptions{}
	cmd := &cobra.Command{
		Use:   "kubeconfig [command]",
		Short: "Create a kubeconfig file for a specified user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.SilenceUsage = true
	cmd.AddCommand(kubeconfigCreateCmd(opts))
	cmd.AddCommand(kubeConfigAdminCmd(opts))
	cmd.PersistentFlags().AddFlagSet(config.GetCLIFlags(opts))
	return cmd
}

var userKubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
apiVersion: v1
clusters:
- cluster:
    server: {{.JoinURL}}
    certificate-authority-data: {{.CACert}}
  name: k0s
contexts:
- context:
    cluster: k0s
    user: {{.User}}
  name: k0s
current-context: k0s
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    client-certificate-data: {{.ClientCert}}
    client-key-data: {{.ClientKey}}
`))

func kubeconfigCreateCmd(opts *config.CLIOptions) *cobra.Command {
	var groups string

	cmd := &cobra.Command{
		Use:   "create username",
		Short: "Create a kubeconfig for a user",
		Long: `Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user`,
		Example: `	Command to create a kubeconfig for a user:
	CLI argument:
	$ k0s kubeconfig create username

	optionally add groups:
	$ k0s kubeconfig create username --groups [groups]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("username is mandatory")
			}
			username := args[0]
			c := k0sGetCmdOpts(opts.DataDir)
			clusterAPIURL := c.NodeConfig.Spec.API.APIAddressURL()

			caCert, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "ca.crt"))
			if err != nil {
				return fmt.Errorf(
					"failed to read cluster ca certificate: %w, check if the control plane is initialized on this node", err)
			}
			caCertPath, caCertKey := path.Join(c.K0sVars.CertRootDir, "ca.crt"), path.Join(c.K0sVars.CertRootDir, "ca.key")
			userReq := certificate.Request{
				Name:   username,
				CN:     username,
				O:      groups,
				CACert: caCertPath,
				CAKey:  caCertKey,
			}
			certManager := certificate.Manager{
				K0sVars: c.K0sVars,
			}
			userCert, err := certManager.EnsureCertificate(userReq, "root")
			if err != nil {
				return err
			}

			data := struct {
				CACert     string
				ClientCert string
				ClientKey  string
				User       string
				JoinURL    string
			}{
				CACert:     base64.StdEncoding.EncodeToString(caCert),
				ClientCert: base64.StdEncoding.EncodeToString([]byte(userCert.Cert)),
				ClientKey:  base64.StdEncoding.EncodeToString([]byte(userCert.Key)),
				User:       username,
				JoinURL:    clusterAPIURL,
			}

			var buf bytes.Buffer

			err = userKubeconfigTemplate.Execute(&buf, &data)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(buf.Bytes())
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&groups, "groups", "", "Specify groups")
	return cmd
}

func kubeConfigAdminCmd(opts *config.CLIOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Display Admin's Kubeconfig file",
		Long:  "Print kubeconfig for the Admin user to stdout",
		Example: `	$ k0s kubeconfig admin > ~/.kube/config
	$ export KUBECONFIG=~/.kube/config
	$ kubectl get nodes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := k0sGetCmdOpts(opts.DataDir)
			content, err := os.ReadFile(c.K0sVars.AdminKubeConfigPath)
			if err != nil {
				return fmt.Errorf("failed to read admin config, check if the control plane is initialized on this node: %w", err)
			}

			clusterAPIURL := c.NodeConfig.Spec.API.APIAddressURL()
			newContent := strings.Replace(string(content), "https://localhost:6443", clusterAPIURL, -1)
			_, err = cmd.OutOrStdout().Write([]byte(newContent))
			return err
		},
	}
	return cmd
}

func k0sGetCmdOpts(dataDir string) k0sconfig.CLIOptions {
	c := k0sconfig.GetCmdOpts()
	k0sVars := constant.GetConfig(filepath.Join(dataDir, "k0s"))
	c.K0sVars = k0sVars
	return c
}
