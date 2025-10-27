package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	autopilot "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	nodeutil "k8s.io/component-helpers/node/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	k0sBinPath = "/usr/local/bin/k0s"
)

type hostInfo struct {
	Hostname         string
	Kclient          client.Client
	KclientError     error
	Node             corev1.Node
	NodeError        error
	ControlNode      autopilot.ControlNode
	ControlNodeError error
	Status           k0s.K0sStatus
	RoleName         string
}

func ResetCmd(ctx context.Context, appTitle string) *cobra.Command {
	var (
		force     bool
		assumeYes bool
	)

	var rc runtimeconfig.RuntimeConfig

	cmd := &cobra.Command{
		Use:   "reset",
		Short: fmt.Sprintf("Remove %s from the current node", appTitle),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("reset command must be run as root")
			}

			rc = rcutil.InitBestRuntimeConfig(cmd.Context())

			_ = rc.SetEnv()

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := maybePrintHAWarning(ctx, rc); err != nil && !force {
				return err
			}

			logrus.Warn("This will remove this node from the cluster and completely reset it, removing all data stored on the node.")
			logrus.Warn("This node will also reboot. Do not reset another node until this is complete.")
			if !force && !assumeYes {
				confirmed, err := prompts.New().Confirm("Do you want to continue?", false)
				if err != nil {
					return fmt.Errorf("failed to get confirmation: %w", err)
				}
				if !confirmed {
					return fmt.Errorf("Aborting")
				}
			}

			// populate options struct with host information
			currentHost, err := newHostInfo(ctx)
			if !checkErrPrompt(assumeYes, force, err) {
				return err
			}

			// basic check to see if it's safe to remove this node from the cluster
			if currentHost.Status.Role == "controller" {
				safeToRemove, reason, err := currentHost.checkResetSafety(ctx, force)
				if !checkErrPrompt(assumeYes, force, err) {
					return err
				}
				if !safeToRemove {
					return fmt.Errorf("%s\nRun reset command with --force to ignore this", reason)
				}
			}

			var numControllerNodes int
			if currentHost.KclientError == nil {
				numControllerNodes, _ = kubeutils.NumOfControlPlaneNodes(ctx, currentHost.Kclient)
			}
			// do not drain node if this is the only controller node in the cluster
			// if there is an error (numControllerNodes == 0), drain anyway to be safe
			if currentHost.Status.Role != "controller" || numControllerNodes != 1 {
				logrus.Info("Draining node...")
				currentHost.drainNode()

				// remove node from cluster
				logrus.Info("Removing node from cluster...")
				removeCtx, removeCancel := context.WithTimeout(ctx, time.Minute)
				defer removeCancel()
				err = currentHost.deleteNode(removeCtx)
				if !checkErrPrompt(assumeYes, force, err) {
					return err
				}

				// controller pre-reset
				if currentHost.Status.Role == "controller" {

					// delete controlNode object from cluster
					deleteControlCtx, deleteCancel := context.WithTimeout(ctx, time.Minute)
					defer deleteCancel()
					err := currentHost.deleteControlNode(deleteControlCtx)
					if !checkErrPrompt(assumeYes, force, err) {
						return err
					}

					// try and leave etcd cluster
					currentHost.leaveEtcdcluster()
				}
			}

			// reset
			logrus.Infof("Resetting node...")
			err = stopAndResetK0s(rc.EmbeddedClusterK0sSubDir())
			if err != nil {
				logrus.Warnf("Failed to stop and reset k0s (continuing with reset anyway): %v", err)
			}

			logrus.Debugf("Resetting firewalld...")
			err = hostutils.ResetFirewalld(ctx)
			if !checkErrPrompt(assumeYes, force, err) {
				return fmt.Errorf("failed to reset firewalld: %w", err)
			}

			if err := helpers.RemoveAll(runtimeconfig.K0sConfigPath); err != nil {
				return fmt.Errorf("failed to remove k0s config: %w", err)
			}

			lamPath := "/etc/systemd/system/local-artifact-mirror.service"
			if _, err := helpers.Stat(lamPath); err == nil {
				if _, err := helpers.RunCommand("systemctl", "stop", "local-artifact-mirror"); err != nil {
					return err
				}
			}
			if err := helpers.RemoveAll(lamPath); err != nil {
				return fmt.Errorf("failed to remove local-artifact-mirror service file: %w", err)
			}

			lamPathD := "/etc/systemd/system/local-artifact-mirror.service.d"
			if err := helpers.RemoveAll(lamPathD); err != nil {
				return fmt.Errorf("failed to remove local-artifact-mirror config directory: %w", err)
			}

			proxyControllerPath := "/etc/systemd/system/k0scontroller.service.d"
			if err := helpers.RemoveAll(proxyControllerPath); err != nil {
				return fmt.Errorf("failed to remove proxy controller config directory: %w", err)
			}

			proxyWorkerPath := "/etc/systemd/system/k0sworker.service.d"
			if err := helpers.RemoveAll(proxyWorkerPath); err != nil {
				return fmt.Errorf("failed to remove proxy worker config directory: %w", err)
			}

			// Now that k0s is nested under the data directory, we see the following error in the
			// dev environment because k0s is mounted in the docker container:
			//  "failed to remove embedded cluster directory: remove k0s: unlinkat /var/lib/embedded-cluster/k0s: device or resource busy"
			if err := helpers.RemoveAll(rc.EmbeddedClusterHomeDirectory()); err != nil {
				logrus.Debugf("Failed to remove embedded cluster directory: %v", err)
			}

			if err := helpers.RemoveAll(runtimeconfig.EmbeddedClusterLogsSubDir()); err != nil {
				return fmt.Errorf("failed to remove logs directory: %w", err)
			}

			if err := helpers.RemoveAll(rc.EmbeddedClusterOpenEBSLocalSubDir()); err != nil {
				return fmt.Errorf("failed to remove openebs storage: %w", err)
			}

			if err := helpers.RemoveAll("/etc/NetworkManager/conf.d/embedded-cluster.conf"); err != nil {
				return fmt.Errorf("failed to remove NetworkManager configuration: %w", err)
			}

			if err := helpers.RemoveAll("/usr/local/bin/k0s"); err != nil {
				return fmt.Errorf("failed to remove k0s binary: %w", err)
			}

			if err := helpers.RemoveAll(runtimeconfig.ECConfigPath); err != nil {
				return fmt.Errorf("failed to remove embedded cluster data config: %w", err)
			}

			if err := helpers.RemoveAll("/etc/sysctl.d/99-embedded-cluster.conf"); err != nil {
				return fmt.Errorf("failed to remove embedded cluster sysctl config: %w", err)
			}

			if _, err := helpers.RunCommand("reboot"); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Ignore errors encountered when resetting the node (implies ---yes)")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	cmd.AddCommand(ResetFirewalldCmd(ctx, appTitle))

	return cmd
}

func checkErrPrompt(noPrompt bool, force bool, err error) bool {
	if err == nil {
		return true
	}
	logrus.Errorf("error: %s", err)

	if force {
		return true
	}

	logrus.Info("An error occurred while trying to reset this node.")

	if noPrompt {
		return false
	}
	logrus.Info("Continuing may leave the cluster in an unexpected state.")
	confirmed, err := prompts.New().Confirm("Do you want to continue anyway?", false)
	if err != nil {
		logrus.Errorf("failed to get confirmation: %v", err)
		return false
	}
	return confirmed
}

// maybePrintHAWarning prints a warning message when the user is running a reset a node
// in a high availability cluster and there are only 3 control nodes.
func maybePrintHAWarning(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	kubeconfig := rc.PathToKubeConfig()
	if _, err := helpers.Stat(kubeconfig); err != nil {
		return nil
	}

	os.Setenv("KUBECONFIG", kubeconfig)
	kubecli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	if in, err := kubeutils.GetLatestInstallation(ctx, kubecli); err != nil {
		if errors.Is(err, kubeutils.ErrNoInstallations{}) {
			return nil // no installations found, not an HA cluster - just an incomplete install
		}

		return fmt.Errorf("unable to get installation: %w", err)
	} else if !in.Spec.HighAvailability {
		return nil
	}

	numControllerNodes, err := kubeutils.NumOfControlPlaneNodes(ctx, kubecli)
	if err != nil {
		return fmt.Errorf("unable to check control plane nodes: %w", err)
	}
	if numControllerNodes == 3 {
		if config.HasCustomRoles() {
			controllerRoleName := config.GetControllerRoleName()
			logrus.Warnf("High-availability is enabled and requires at least three %s nodes.", controllerRoleName)
			logrus.Warn("Resetting this node will leave only two.")
			logrus.Warn("This can lead to a loss of functionality and non-recoverable failures.")
			logrus.Warnf("If you reset this node, re-join a third %s node as soon as possible.", controllerRoleName)
		} else {
			logrus.Warn("High-availability is enabled and requires at least three nodes.")
			logrus.Warn("Resetting this node will leave only two.")
			logrus.Warn("This can lead to a loss of functionality and non-recoverable failures.")
			logrus.Warn("If you reset this node, re-join a third node as soon as possible.")
		}
		logrus.Info("")
	}
	return nil
}

// newHostInfo returns a populated hostInfo struct
func newHostInfo(ctx context.Context) (hostInfo, error) {
	currentHost := hostInfo{}
	// populate hostname
	err := currentHost.getHostName()
	if err != nil {
		err = fmt.Errorf("unable to get hostname: %w", err)
		currentHost.KclientError = err
		return currentHost, err
	}
	// get k0s status json
	status, err := k0s.GetStatus(ctx)
	if err != nil {
		err := fmt.Errorf("client not initialized")
		currentHost.KclientError = err
		return currentHost, err
	}
	currentHost.Status = *status
	currentHost.RoleName = currentHost.Status.Role
	// set up kube client
	currentHost.configureKubernetesClient()
	// fetch node object
	currentHost.getNodeObject(ctx)
	// control plane only stuff
	if currentHost.Status.Role == "controller" {
		// fetch controlNode
		currentHost.getControlNodeObject(ctx)
	}
	// try and get custom role name from the node labels
	labels := currentHost.Node.GetLabels()
	if value, ok := labels["kots.io/embedded-cluster-role-0"]; ok {
		currentHost.RoleName = value
	}
	return currentHost, nil
}

type etcdMembers struct {
	Members map[string]string `json:"members"`
}

// leaveEtcdcluster uses k0s to attempt to leave the etcd cluster
func (h *hostInfo) leaveEtcdcluster() {
	// Check if k0s binary exists
	if _, err := helpers.Stat(k0sBinPath); os.IsNotExist(err) {
		logrus.Debugf("k0s binary not found at %s, skipping etcd leave", k0sBinPath)
		return
	}

	// Try to list members with retries
	var memberlist etcdMembers
	var out string
	var err error

	// Retry member list up to 3 times
	for i := 0; i < 3; i++ {
		out, err = helpers.RunCommand(k0sBinPath, "etcd", "member-list")
		if err == nil {
			err = json.Unmarshal([]byte(out), &memberlist)
			if err == nil {
				break
			}
		}
		if i < 2 { // Don't sleep on last attempt
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		logrus.Warnf("Unable to list etcd members, continuing with reset: %v", err)
		return
	}

	// If we're the only member, no need to leave
	if len(memberlist.Members) == 1 && memberlist.Members[h.Hostname] != "" {
		return
	}

	// Attempt to leave the cluster with retries
	for i := 0; i < 3; i++ {
		out, err = helpers.RunCommand(k0sBinPath, "etcd", "leave")
		if err == nil {
			return
		}

		// Check if the error is due to etcd being stopped
		if strings.Contains(err.Error(), "etcdserver: server stopped") {
			logrus.Warnf("Etcd server is stopped, continuing with reset")
			return
		}

		if i < 2 { // Don't sleep on last attempt
			time.Sleep(2 * time.Second)
		}
	}

	// If we get here, we failed to leave after retries
	logrus.Warnf("Unable to leave etcd cluster after retries (this is often normal during reset): %v, %s", err, out)
}

var (
	notFoundRegex = regexp.MustCompile(`nodes ".+" not found`)
)

// drainNode uses k0s to initiate a node drain
func (h *hostInfo) drainNode() {
	// Check if k0s binary exists
	if _, err := helpers.Stat(k0sBinPath); os.IsNotExist(err) {
		logrus.Debugf("k0s binary not found at %s, skipping node drain", k0sBinPath)
		return
	}

	os.Setenv("KUBECONFIG", h.Status.Vars.KubeletAuthConfigPath)
	drainArgList := []string{
		"kubectl",
		"drain",
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		"--timeout", "60s",
		h.Hostname,
	}
	out, err := helpers.RunCommand(k0sBinPath, drainArgList...)
	if err != nil {
		if notFoundRegex.Match([]byte(out + err.Error())) {
			return
		}
		// Log the error but continue with reset
		logrus.Warnf("Node drain failed (continuing with reset anyway): %v, %s", err, out)
		return
	}
}

// configureKubernetesClient optimistically sets up a client to use for kubernetes api calls
// it stores any errors in h.KclientError
func (h *hostInfo) configureKubernetesClient() {
	os.Setenv("KUBECONFIG", h.Status.Vars.KubeletAuthConfigPath)
	client, err := kubeutils.KubeClient()
	if err != nil {
		h.KclientError = fmt.Errorf("unable to create kube client: %w", err)
		return
	}
	h.Kclient = client
}

// getHostName fetches the hostname for the node
func (h *hostInfo) getHostName() error {
	hostname, err := nodeutil.GetHostname("")
	if err != nil {
		return fmt.Errorf("unable to get hostname: %w", err)
	}
	h.Hostname = hostname
	return nil
}

// getNodeObject optimistically fetches the node object from the k8s api server
// it stores any errors in h.NodeError
func (h *hostInfo) getNodeObject(ctx context.Context) {
	if h.KclientError != nil {
		h.NodeError = fmt.Errorf("unable to load cluster client: %w", h.KclientError)
		return
	}
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.Node)
	if err != nil {
		h.NodeError = fmt.Errorf("unable to get Node: %w", err)
		return
	}
}

// getControlNodeObject optimistically fetches the controlNode object from the k8s api server
// it stores any errors in h.ControlNodeError
func (h *hostInfo) getControlNodeObject(ctx context.Context) {
	if h.KclientError != nil {
		h.ControlNodeError = fmt.Errorf("unable to load cluster client: %w", h.KclientError)
		return
	}
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.ControlNode)
	if err != nil {
		h.ControlNodeError = fmt.Errorf("unable to get ControlNode: %w", err)
		return
	}
}

// checkResetSafety performs checks to see if the reset would cause an outage
func (h *hostInfo) checkResetSafety(ctx context.Context, force bool) (bool, string, error) {
	if force {
		return true, "", nil
	}

	if h.KclientError != nil {
		return false, "", fmt.Errorf("unable to load cluster client: %w", h.KclientError)
	}

	etcdClient, err := etcd.NewClient(h.Status.Vars.CertRootDir, h.Status.Vars.EtcdCertDir, h.Status.ClusterConfig.Spec.Storage.Etcd)
	if err != nil {
		return false, "", fmt.Errorf("unable to create etcd client: %w", err)
	}
	if etcdClient.Health(ctx) != nil {
		return false, "Etcd is not ready. Please wait up to 5 minutes and try again.", nil
	}

	// get a rough picture of the cluster topology
	workers := []string{}
	controllers := []string{}
	nodeList := corev1.NodeList{}
	err = h.Kclient.List(ctx, &nodeList)
	if err != nil {
		return false, "", fmt.Errorf("unable to list Nodes: %w", err)
	}
	for _, node := range nodeList.Items {
		labels := node.GetLabels()
		if _, ok := labels["node-role.kubernetes.io/control-plane"]; ok {
			controllers = append(controllers, node.Name)
		} else {
			workers = append(workers, node.Name)
		}
	}
	if len(workers) > 0 && len(controllers) == 1 {
		message := fmt.Sprintf("Cannot reset the last %s node when there are other nodes in the cluster.", h.RoleName)
		return false, message, nil
	}
	return true, "", nil
}

// deleteControlNode removes the controlNode object from the cluster
func (h *hostInfo) deleteControlNode(ctx context.Context) error {
	if h.KclientError != nil {
		return fmt.Errorf("unable to delete ControlNode: %w", h.KclientError)
	}
	if h.ControlNodeError != nil {
		if k8serrors.IsNotFound(h.ControlNodeError) {
			return nil
		}
		return fmt.Errorf("unable to delete ControlNode: %w", h.ControlNodeError)
	}
	err := h.Kclient.Delete(ctx, &h.ControlNode)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("unable to delete ControlNode: %w", err)
	}
	return nil
}

// deleteNode removes the node from the cluster
func (h *hostInfo) deleteNode(ctx context.Context) error {
	if h.KclientError != nil {
		return fmt.Errorf("unable to delete Node: %w", h.KclientError)
	}
	if h.NodeError != nil {
		if k8serrors.IsNotFound(h.NodeError) {
			return nil
		}
		return fmt.Errorf("unable to delete Node: %w", h.NodeError)
	}
	err := h.Kclient.Delete(ctx, &h.Node)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("unable to delete Node: %w", err)
	}
	return nil
}

// stopK0s attempts to stop the k0s service
func stopAndResetK0s(dataDir string) error {
	// Check if k0s binary exists
	if _, err := helpers.Stat(k0sBinPath); os.IsNotExist(err) {
		logrus.Debugf("k0s binary not found at %s, skipping k0s stop and reset", k0sBinPath)
		return nil
	}

	// Check if k0s services exist
	k0sControllerExists := false
	k0sWorkerExists := false

	if out, err := helpers.RunCommand("systemctl", "list-unit-files", "k0scontroller.service"); err == nil && strings.Contains(out, "k0scontroller.service") {
		k0sControllerExists = true
	}

	if out, err := helpers.RunCommand("systemctl", "list-unit-files", "k0sworker.service"); err == nil && strings.Contains(out, "k0sworker.service") {
		k0sWorkerExists = true
	}

	if !k0sControllerExists && !k0sWorkerExists {
		logrus.Debugf("No k0s services found, skipping k0s stop and reset")
		return nil
	}

	out, err := helpers.RunCommand(k0sBinPath, "stop")
	if err != nil {
		return fmt.Errorf("could not stop k0s service: %w, %s", err, out)
	}
	out, err = helpers.RunCommand(k0sBinPath, "reset", "--data-dir", dataDir)
	if err != nil {
		return fmt.Errorf("could not reset k0s: %w, %s", err, out)
	}
	return nil
}
