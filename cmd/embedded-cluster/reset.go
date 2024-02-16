package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	autopilot "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

type etcdMembers struct {
	Members map[string]string `json:"members"`
}

type hostInfo struct {
	Hostname         string
	Kclient          client.Client
	KclientError     error
	Node             corev1.Node
	NodeError        error
	ControlNode      autopilot.ControlNode
	ControlNodeError error
	Status           k0sStatus
	RoleName         string
}

type k0sStatus struct {
	Role          string                `json:"Role"`
	Vars          k0sVars               `json:"K0sVars"`
	ClusterConfig v1beta1.ClusterConfig `json:"ClusterConfig"`
}

type k0sVars struct {
	KubeletAuthConfigPath string `json:"KubeletAuthConfigPath"`
	CertRootDir           string `json:"CertRootDir"`
	EtcdCertDir           string `json:"EtcdCertDir"`
}

var (
	binName = defaults.BinaryName()
	k0s     = "/usr/local/bin/k0s"
)

// deleteNode removes the node from the cluster
func (h *hostInfo) deleteNode(ctx context.Context) error {
	if h.KclientError != nil {
		return fmt.Errorf("unable to delete Node: %w", h.KclientError)
	}
	if h.NodeError != nil {
		return fmt.Errorf("unable to delete Node: %w", h.NodeError)
	}
	err := h.Kclient.Delete(ctx, &h.Node)
	if err != nil {
		return fmt.Errorf("unable to delete Node: %w", err)
	}
	return nil
}

// deleteControlNode removes the controlNode object from the cluster
func (h *hostInfo) deleteControlNode(ctx context.Context) error {
	if h.KclientError != nil {
		return fmt.Errorf("unable to delete ControlNode: %w", h.KclientError)
	}
	if h.ControlNodeError != nil {
		return fmt.Errorf("unable to delete ControlNode: %w", h.ControlNodeError)
	}
	err := h.Kclient.Delete(ctx, &h.ControlNode)
	if err != nil {
		return fmt.Errorf("unable to delete ControlNode: %w", err)
	}
	return nil
}

// drainNode uses k0s to initiate a node drain
func (h *hostInfo) drainNode() error {
	os.Setenv("KUBECONFIG", h.Status.Vars.KubeletAuthConfigPath)
	drainArgList := []string{
		"kubectl",
		"drain",
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		h.Hostname,
	}
	out, err := exec.Command(k0s, drainArgList...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not drain node: %w, %s", err, out)
	}
	return nil
}

// configureKubernetesClient optimistically sets up a client to use for kubernetes api calls
// it stores any errors in h.KclientError
func (h *hostInfo) configureKubernetesClient() {
	os.Setenv("KUBECONFIG", h.Status.Vars.KubeletAuthConfigPath)
	config, err := controllerruntime.GetConfig()
	if err != nil {
		h.KclientError = fmt.Errorf("unable to create cluster client config: %w", err)
		return
	}
	h.Kclient, err = client.New(config, client.Options{})
	if err != nil {
		h.KclientError = fmt.Errorf("unable to create cluster client: %w", err)
		return
	}
	autopilot.AddToScheme(h.Kclient.Scheme())
	v1beta1.AddToScheme(h.Kclient.Scheme())
}

// getHostName fetches the hostname for the node
func (h *hostInfo) getHostName() error {
	hostname, err := os.Hostname()
	if err != nil {
		return nil
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
func (h *hostInfo) checkResetSafety(c *cli.Context) (bool, string, error) {
	if c.Bool("force") {
		return true, "", nil
	}

	if h.KclientError != nil {
		return false, "", fmt.Errorf("unable to load cluster client: %w", h.KclientError)
	}

	etcdClient, err := etcd.NewClient(h.Status.Vars.CertRootDir, h.Status.Vars.EtcdCertDir, h.Status.ClusterConfig.Spec.Storage.Etcd)
	if err != nil {
		return false, "", fmt.Errorf("unable to create etcd client: %w", err)
	}
	if etcdClient.Health(c.Context) != nil {
		return false, "Etcd is not ready. Please wait up to 5 minutes and try again.", nil
	}

	// get a rough picture of the cluster topology
	workers := []string{}
	controllers := []string{}
	nodeList := corev1.NodeList{}
	err = h.Kclient.List(c.Context, &nodeList)
	if err != nil {
		return false, "", fmt.Errorf("unable to list Nodes: %w", err)
	}
	for _, node := range nodeList.Items {
		labels := node.GetLabels()
		if labels["node-role.kubernetes.io/control-plane"] == "true" {
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

// leaveEtcdcluster uses k0s to attempt to leave the etcd cluster
func (h *hostInfo) leaveEtcdcluster() error {

	// if we're the only etcd member we don't need to leave the cluster
	out, err := exec.Command(k0s, "etcd", "member-list").Output()
	if err != nil {
		return err
	}
	memberlist := etcdMembers{}
	err = json.Unmarshal(out, &memberlist)
	if err != nil {
		return err
	}
	if len(memberlist.Members) == 1 && memberlist.Members[h.Hostname] != "" {
		return nil
	}

	out, err = exec.Command(k0s, "etcd", "leave").CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to leave etcd cluster: %w, %s", err, string(out))
	}
	return nil
}

// stopK0s attempts to stop the k0s service
func stopAndResetK0s() error {
	out, err := exec.Command(k0s, "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not stop k0s service: %w, %s", err, string(out))
	}
	out, err = exec.Command(k0s, "reset").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not reset k0s: %w, %s", err, string(out))
	}
	logrus.Infof("Node has been reset. Please reboot to ensure transient configuration is also reset.")
	return nil
}

// newHostInfo returns a populated hostInfo struct
func newHostInfo(c *cli.Context) (hostInfo, error) {
	currentHost := hostInfo{}
	// populate hostname
	err := currentHost.getHostName()
	if err != nil {
		currentHost.KclientError = fmt.Errorf("client not initialized")
		return currentHost, err
	}
	// get k0s status json
	out, err := exec.Command(k0s, "status", "-o", "json").Output()
	if err != nil {
		currentHost.KclientError = fmt.Errorf("client not initialized")
		return currentHost, err
	}
	err = json.Unmarshal(out, &currentHost.Status)
	if err != nil {
		currentHost.KclientError = fmt.Errorf("client not initialized")
		return currentHost, err
	}
	currentHost.RoleName = currentHost.Status.Role
	// set up kube client
	currentHost.configureKubernetesClient()
	// fetch node object
	currentHost.getNodeObject(c.Context)
	// control plane only stff
	if currentHost.Status.Role == "controller" {
		// fetch controlNode
		currentHost.getControlNodeObject(c.Context)
	}
	// try and get custom role name from the node labels
	labels := currentHost.Node.GetLabels()
	if value, ok := labels["kots.io/embedded-cluster-role-0"]; ok {
		currentHost.RoleName = value
	}
	return currentHost, nil
}

func checkErrPrompt(c *cli.Context, err error) bool {
	if err == nil {
		return true
	}
	logrus.Errorf("error: %s", err)
	if c.Bool("force") {
		return true
	}
	logrus.Info("An error occurred while trying to reset this node.")
	if c.Bool("no-prompt") {
		return false
	}
	logrus.Info("Continuing may leave the cluster in an unexpected state.")
	return prompts.New().Confirm("Do you want to continue anyway?", false)
}

var resetCommand = &cli.Command{
	Name: "reset",
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("node reset command must be run as root")
		}
		return nil
	},
	Args: false,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Disable interactive prompts",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Ignore errors encountered when resetting the node (implies --no-prompt)",
			Value: false,
		},
	},
	Usage: "Reset the current node",
	Action: func(c *cli.Context) error {
		logrus.Info("This will remove this node from the cluster and completely reset it.")
		logrus.Info("Do not reset another node until this reset is complete.")
		if !c.Bool("force") && !c.Bool("no-prompt") && !prompts.New().Confirm("Do you want to continue?", false) {
			return fmt.Errorf("Aborting")
		}

		// populate options struct with host information
		currentHost, err := newHostInfo(c)
		if !checkErrPrompt(c, err) {
			return err
		}

		// basic check to see if it's safe to remove this node from the cluster
		if currentHost.Status.Role == "controller" {
			safeToRemove, reason, err := currentHost.checkResetSafety(c)
			if !checkErrPrompt(c, err) {
				return err
			}
			if !safeToRemove {
				return fmt.Errorf("%s\nRun reset command with --force to ignore this", reason)
			}
		}

		// drain node
		logrus.Info("Draining node...")
		err = currentHost.drainNode()
		if !checkErrPrompt(c, err) {
			return err
		}

		// remove node from cluster
		logrus.Info("Removing node from cluster...")
		err = currentHost.deleteNode(c.Context)
		if !checkErrPrompt(c, err) {
			return err
		}

		// controller pre-reset
		if currentHost.Status.Role == "controller" {

			// delete controlNode object from cluster
			err := currentHost.deleteControlNode(c.Context)
			if !checkErrPrompt(c, err) {
				return err
			}

			// try and leave etcd cluster
			err = currentHost.leaveEtcdcluster()
			if !checkErrPrompt(c, err) {
				return err
			}

		}

		// reset
		logrus.Infof("Resetting %s...", binName)
		err = stopAndResetK0s()
		if !checkErrPrompt(c, err) {
			return err
		}

		if _, err := os.Stat(defaults.PathToK0sConfig()); err == nil {
			if err := os.Remove(defaults.PathToK0sConfig()); err != nil {
				return err
			}
		}

		if _, err := os.Stat(defaults.EmbeddedClusterHomeDirectory()); err == nil {
			if err := os.RemoveAll(defaults.EmbeddedClusterHomeDirectory()); err != nil {
				return err
			}
		}

		return nil
	},
}
