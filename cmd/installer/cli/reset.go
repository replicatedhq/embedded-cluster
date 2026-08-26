package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
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

// k0sBinPath is the k0s binary this command stops and resets. Overridden by tests.
var k0sBinPath = "/usr/local/bin/k0s"

const (
	// k0sRunDir holds k0s runtime state: the containerd socket, sandbox shm
	// mounts and task rootfs overlays.
	k0sRunDir = "/run/k0s"
)

// k0sResetTimeout bounds the `k0s reset` call. k0s issues its container-runtime
// calls with no deadline, so a containerd that stops responding hangs reset
// forever and none of the teardown after it ever runs. Generous enough not to
// cut short a slow disk. Overridden by tests.
var k0sResetTimeout = 2 * time.Minute

// k0sResetDumpLead is how long before the deadline k0s is asked for a goroutine
// dump, leaving it time to print the stacks before the deadline kills it.
var k0sResetDumpLead = 15 * time.Second

//go:embed assets/unmount.sh
var unmountScript string

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
				if err != nil {
					if k8serrors.IsForbidden(err) && currentHost.Status.Role == "worker" {
						logrus.Warnf("Unable to delete this worker node from the API server due to insufficient permissions.")
						logrus.Infof("To complete the reset, remove this node from the cluster by running 'kubectl delete node %s' from a surviving controller node.", currentHost.Hostname)
						if !force && !assumeYes {
							confirmed, promptErr := prompts.New().Confirm("Do you want to continue with the local reset anyway?", false)
							if promptErr != nil {
								return fmt.Errorf("failed to get confirmation: %w", promptErr)
							}
							if !confirmed {
								return fmt.Errorf("reset aborted")
							}
						}
					} else if !checkErrPrompt(assumeYes, force, err) {
						return err
					}
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
					currentHost.leaveEtcdCluster()
				}
			}

			// reset
			logrus.Infof("Resetting node...")
			err = stopAndResetK0s(rc.EmbeddedClusterK0sSubDir())
			if err != nil {
				logrus.Warnf("Failed to stop and reset k0s (continuing with reset anyway): %v", err)
				// k0s did not finish its own cleanup, so nothing killed the processes
				// holding the kubelet and containerd mounts or detached them. Without
				// this the removals below fail with EBUSY and the node is left with an
				// installation the next install refuses to overwrite.
				forceK0sTeardown(rc.EmbeddedClusterHomeDirectory(), rc.EmbeddedClusterK0sSubDir())
			}

			logrus.Debugf("Resetting firewalld...")
			err = hostutils.ResetFirewalld(ctx)
			if !checkErrPrompt(assumeYes, force, err) {
				return fmt.Errorf("failed to reset firewalld: %w", err)
			}

			// The whole directory, not just k0s.yaml: it also holds containerd
			// drop-ins and registry certs, and nothing else removes them — k0s's
			// own cleanup only covers its data and run dirs.
			if err := helpers.RemoveAll(filepath.Dir(runtimeconfig.K0sConfigPath)); err != nil {
				return fmt.Errorf("failed to remove k0s config directory: %w", err)
			}

			lamPath := "/etc/systemd/system/local-artifact-mirror.service"
			if _, err := os.Stat(lamPath); err == nil {
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

			// Remove k0s unit files explicitly in case k0s reset failed to do so.
			if err := helpers.RemoveAll("/etc/systemd/system/k0scontroller.service"); err != nil {
				return fmt.Errorf("failed to remove k0scontroller service file: %w", err)
			}
			if err := helpers.RemoveAll("/etc/systemd/system/k0sworker.service"); err != nil {
				return fmt.Errorf("failed to remove k0sworker service file: %w", err)
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
	if _, err := os.Stat(kubeconfig); err != nil {
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

// leaveEtcdCluster uses k0s to attempt to leave the etcd cluster
func (h *hostInfo) leaveEtcdCluster() {
	// Check if k0s binary exists
	if _, err := os.Stat(k0sBinPath); os.IsNotExist(err) {
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
	if _, err := os.Stat(k0sBinPath); os.IsNotExist(err) {
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
	// Controllers have admin kubeconfig with cluster-admin permissions.
	// Workers only have kubelet auth config, so fall back when admin.conf is absent.
	kubeconfigPath := h.Status.Vars.KubeletAuthConfigPath
	if _, err := os.Stat(h.Status.Vars.AdminKubeConfigPath); err == nil {
		kubeconfigPath = h.Status.Vars.AdminKubeConfigPath
	}
	os.Setenv("KUBECONFIG", kubeconfigPath)
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
	if _, err := os.Stat(k0sBinPath); os.IsNotExist(err) {
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

	stopOut := &lineLogWriter{prefix: "k0s stop"}
	err := helpers.RunCommandWithOptions(helpers.RunCommandOptions{Stdout: stopOut, Stderr: stopOut, SkipLogOutput: true}, k0sBinPath, "stop", "--verbose")
	if err != nil {
		// k0s reset must still run to unmount kubelet pod-volume mounts.
		logrus.Warnf("Failed to stop k0s (continuing with reset anyway): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), k0sResetTimeout)
	defer cancel()

	// Shortly before the deadline, ask k0s for a goroutine dump. SIGQUIT makes it
	// print every stack and exit, and its output is streamed to the log, so the
	// stuck call (in practice a CRI RemovePodSandbox) is recorded. Signalling
	// containerd instead yields nothing: its output is piped to this k0s process,
	// which the deadline is about to kill.
	var dumped atomic.Bool
	dump := time.AfterFunc(k0sResetTimeout-k0sResetDumpLead, func() {
		dumped.Store(true)
		logrus.Warnf("k0s reset is taking too long, collecting a stack dump")
		_, _ = helpers.RunCommand("pkill", "-QUIT", "-f", "k0s reset --data-dir")
	})
	defer dump.Stop()

	resetOut := &lineLogWriter{prefix: "k0s reset"}
	err = helpers.RunCommandWithOptions(helpers.RunCommandOptions{Context: ctx, Stdout: resetOut, Stderr: resetOut, SkipLogOutput: true}, k0sBinPath, "reset", "--data-dir", dataDir, "--verbose")
	if err != nil {
		if ctx.Err() != nil || dumped.Load() {
			return fmt.Errorf("k0s reset timed out after %s: %w", k0sResetTimeout, err)
		}
		return fmt.Errorf("could not reset k0s: %w", err)
	}
	return nil
}

// forceK0sTeardown does the cleanup `k0s reset` would have done itself: kill the
// processes still holding the kubelet and containerd mounts, then detach the
// mounts so the directories can be removed. Every step is best-effort — pkill
// exits non-zero when nothing matches, which is the normal case.
func forceK0sTeardown(homeDir, k0sDataDir string) {
	logrus.Infof("Force killing any stale k0s processes")
	for _, proc := range []string{"k0s", "kube-apiserver", "kube-controller-manager", "kube-scheduler", "kubelet", "containerd"} {
		_, _ = helpers.RunCommand("pkill", "-9", "-f", proc)
	}

	// homeDir is listed as well because reset removes it whole, and it is not
	// always a parent of k0sDataDir — K0sDataDirOverride moves the latter out.
	for _, dir := range []string{homeDir, k0sDataDir, k0sRunDir} {
		if out, err := helpers.RunCommand("sh", "-c", unmountScript, "sh", dir); err != nil {
			logrus.Debugf("Failed to unmount below %s (ignored): %v, %s", dir, err, out)
		}
	}

	// Remove vxlan.calico (holds port 4789/UDP), the Calico veth interfaces and
	// the blackhole routes so the pod CIDR can be reused. k0s does this in its own
	// cni cleanup step, which is one of the steps that did not run.
	for _, cmd := range [][]string{
		{"ip", "link", "delete", "vxlan.calico"},
		{"sh", "-c", "ip link show | grep -oE ' cali[0-9a-f]+' | xargs -r -L1 ip link delete"},
		{"sh", "-c", "ip route show table all | grep blackhole | grep 'proto 80' | awk '{print $1, $2}' | xargs -r -L1 ip route delete"},
	} {
		_, _ = helpers.RunCommand(cmd[0], cmd[1:]...)
	}
}

// lineLogWriter streams command output to logrus as it is written, so that
// partial output is captured in the logs even if the command is interrupted.
type lineLogWriter struct {
	prefix string
}

func (w *lineLogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			logrus.Debugf("%s: %s", w.prefix, line)
		}
	}
	return len(p), nil
}
