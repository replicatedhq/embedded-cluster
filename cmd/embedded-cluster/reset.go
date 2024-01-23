package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	autopilot "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
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
	Hostname    string
	Kclient     client.Client
	Node        corev1.Node
	ControlNode autopilot.ControlNode
	Status      k0sStatus
	RoleName    string
}

type k0sStatus struct {
	Role string  `json:"Role"`
	Vars k0sVars `json:"K0sVars"`
}

type k0sVars struct {
	KubeletAuthConfigPath string `json:"KubeletAuthConfigPath"`
}

var (
	binName = defaults.BinaryName()
	k0s     = defaults.K0sBinaryPath()
)

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

// configureKubernetesClient sets up a client to use for kubernetes api calls
func (h *hostInfo) configureKubernetesClient() error {
	os.Setenv("KUBECONFIG", h.Status.Vars.KubeletAuthConfigPath)
	config, err := controllerruntime.GetConfig()
	if err != nil {
		return err
	}
	h.Kclient, err = client.New(config, client.Options{})
	autopilot.AddToScheme(h.Kclient.Scheme())
	if err != nil {
		return fmt.Errorf("couldn't create k8s config: %w", err)
	}
	return nil
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

// getNodeObject fetches the node object from the k8s api server
func (h *hostInfo) getNodeObject(ctx context.Context) error {
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.Node)
	if err != nil {
		return err
	}
	return nil
}

// getControlNodeObject fetches the controlNode object from the k8s api server
func (h *hostInfo) getControlNodeObject(ctx context.Context) error {
	err := h.Kclient.Get(ctx, client.ObjectKey{Name: h.Hostname}, &h.ControlNode)
	if err != nil {
		return err
	}
	return nil
}

func (h *hostInfo) checkQuorumSafety(c *cli.Context) (bool, string, error) {
	if c.Bool("yes-really-reset") {
		return true, "", nil
	}
	out, err := exec.Command(k0s, "etcd", "member-list").Output()
	if err != nil {
		return false, "", fmt.Errorf("unable to fetch etcd member list, %w, %s", err, out)
	}
	etcd := etcdMembers{}
	err = json.Unmarshal(out, &etcd)
	if err != nil {
		return false, "", fmt.Errorf("unable to unmarshal etcd member list, %w, %s", err, out)
	}
	// get a rough picture of the cluster topology
	workers := []string{}
	controllers := []string{}
	nodeList := corev1.NodeList{}
	err = h.Kclient.List(c.Context, &nodeList)
	if err != nil {
		return false, "", fmt.Errorf("unable to create kubernetes client: %w", err)
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
	if len(etcd.Members) < 3 {
		return true, "", nil
	}
	if len(etcd.Members) == 3 {
		message := fmt.Sprintf("Cluster has 3 %s nodes. Removing this node will cause etcd to lose quorum.", h.RoleName)
		return false, message, nil
	}
	if len(etcd.Members)%2 != 0 {
		message := fmt.Sprintf("This will leave the cluster with an even number of %s nodes, which could make it unstable.", h.RoleName)
		return false, message, nil
	}
	return true, "", nil
}

// leaveEtcdcluster uses k0s to attempt to leave the etcd cluster
func (h *hostInfo) leaveEtcdcluster() error {
	out, err := exec.Command(k0s, "etcd", "leave").CombinedOutput()
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
	fmt.Println("Node has been reset. Please reboot to ensure transient configuration is also reset.")
	return nil
}

// newHostInfo returns a populated hostInfo struct
func newHostInfo(ctx context.Context) (hostInfo, error) {
	currentHost := hostInfo{}
	// populate hostname
	err := currentHost.getHostName()
	if err != nil {
		return currentHost, err
	}
	// get k0s status
	out, err := exec.Command(k0s, "status", "-o", "json").Output()
	if err != nil {
		return currentHost, fmt.Errorf("unable to fetch k0s status, %w, %s", err, out)
	}
	err = json.Unmarshal(out, &currentHost.Status)
	if err != nil {
		return currentHost, fmt.Errorf("unable to unmarshal k0s status, %w, %s", err, out)
	}
	// set up kube client
	err = currentHost.configureKubernetesClient()
	if err != nil {
		return currentHost, err
	}
	// fetch node object
	err = currentHost.getNodeObject(ctx)
	if err != nil {
		return currentHost, err
	}
	currentHost.RoleName = "worker"
	// control plane only stff
	if currentHost.Status.Role == "controller" {
		currentHost.RoleName = "controler"
		// fetch controlNode
		err := currentHost.getControlNodeObject(ctx)
		if err != nil {
			return currentHost, err
		}
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
	fmt.Println("-----")
	fmt.Println(err)
	fmt.Println("-----")
	fmt.Println("An error occurred while trying to reset this node.")
	fmt.Println("Continuing may leave the cluster in an unexpected state.")
	if c.Bool("no-prompt") {
		return true
	}
	return prompts.New().Confirm("Do you want to continue anyway?", false)
}

var resetCommand = &cli.Command{
	Name: "reset",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:   "confirm",
			Hidden: true,
			Value:  false,
		},
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
		&cli.BoolFlag{
			Name:   "force",
			Hidden: true,
			Value:  false,
		},
	},
	Usage: "Reset the current node",
	Action: func(c *cli.Context) error {

		if c.Bool("force") {
			err := stopAndResetK0s()
			if err != nil {
				fmt.Println(err)
				return nil
			}
			return nil
		}

		fmt.Println("This will remove this node from the cluster and completely reset it.")
		if !c.Bool("no-prompt") && !prompts.New().Confirm("Do you want to continue?", false) {
			fmt.Println("Aborting.")
			return nil
		}

		// populate options struct with host information
		currentHost, err := newHostInfo(c.Context)
		if !checkErrPrompt(c, err) {
			return nil
		}

		// basic check to see if it's safe to remove this node from the cluster
		if currentHost.Status.Role == "controller" {
			safeToRemove, reason, err := currentHost.checkQuorumSafety(c)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			if !safeToRemove {
				fmt.Println(reason)
				fmt.Println("Run reset command with --confirm to ignore this.")
				return nil
			}
		}

		// determine if this is the only node in the cluster
		// if this is a single node we can skip a lot of steps
		nodeList := corev1.NodeList{}
		currentHost.Kclient.List(c.Context, &nodeList)
		if len(nodeList.Items) == 1 {
			nodeName := nodeList.Items[0].Name
			if nodeName != currentHost.Hostname {
				fmt.Println("Detected a single-node cluster, but the node's name doesn't match our hostname.")
				return nil
			}
			// stop k0s
			fmt.Printf("Resetting %s...\n", binName)
			err = stopAndResetK0s()
			if !checkErrPrompt(c, err) {
				return nil
			}
			return nil
		}

		// drain node
		fmt.Println("Draining node...")
		err = currentHost.drainNode()
		if !checkErrPrompt(c, err) {
			return nil
		}

		// remove node from cluster
		fmt.Println("Removing node from cluster...")
		err = currentHost.Kclient.Delete(c.Context, &currentHost.Node)
		if !checkErrPrompt(c, err) {
			return nil
		}

		// controller pre-reset
		if currentHost.Status.Role == "controller" {

			// delete controlNode object from cluster
			err := currentHost.Kclient.Delete(c.Context, &currentHost.ControlNode)
			if !checkErrPrompt(c, err) {
				return nil
			}

			// try and leave etcd cluster
			err = currentHost.leaveEtcdcluster()
			if !checkErrPrompt(c, err) {
				return nil
			}

		} else if err != nil {
			fmt.Println(err)
			return nil
		}

		// reset
		fmt.Printf("Resetting %s...\n", binName)
		err = stopAndResetK0s()
		if !checkErrPrompt(c, err) {
			return nil
		}

		return nil
	},
}
