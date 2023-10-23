package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/google/uuid"

	"github.com/replicatedhq/embedded-cluster/e2e/scripts"
)

var networkaddr chan string

const lxdSocket = "/var/snap/lxd/common/lxd/unix.socket"
const profileConfig = `lxc.apparmor.profile=unconfined
lxc.cap.drop=
lxc.cgroup.devices.allow=a
lxc.mount.auto=proc:rw sys:rw
lxc.mount.entry = /dev/kmsg dev/kmsg none defaults,bind,create=file`
const checkInternet = `#!/bin/bash
timeout 5 bash -c 'cat < /dev/null > /dev/tcp/www.replicated.com/80'
if [ $? == 0 ]; then
    exit 0
fi
echo "Internet connection is down"
exit 1
`

func init() {
	networkaddr = make(chan string, 255)
	for i := 2; i < 255; i++ {
		networkaddr <- fmt.Sprintf("192.168.%d", i)
	}
}

// Input are the options passed in to the cluster creation plus some data
// for internal consumption only.
type Input struct {
	Nodes         int
	SSHPublicKey  string
	SSHPrivateKey string
	HelmVMPath    string
	Image         string
	network       string
	T             *testing.T
	id            string
}

// File holds information about a file that must be uploaded to a node.
type File struct {
	SourcePath string
	DestPath   string
	Mode       int
}

// Output is returned when a cluster is created. Contain a list of all node
// names and the cluster id.
type Output struct {
	Nodes   []string
	network string
	id      string
	T       *testing.T
}

// Destroy destroys a cluster pointed by the id property inside the output.
func (o *Output) Destroy() {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		o.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	for _, node := range o.Nodes {
		reqstate := api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
		}
		op, err := client.UpdateInstanceState(node, reqstate, "")
		if err != nil {
			o.T.Logf("Failed to stop node %s: %v", node, err)
			continue
		}
		if err := op.Wait(); err != nil {
			o.T.Logf("Failed to wait node %s to stop: %v", node, err)
		}
	}
	netname := fmt.Sprintf("internal-%s", o.id)
	if err := client.DeleteNetwork(netname); err != nil {
		o.T.Logf("Failed to delete network %s: %v", netname, err)
	}
	netname = fmt.Sprintf("external-%s", o.id)
	if err := client.DeleteNetwork(netname); err != nil {
		o.T.Logf("Failed to delete external network: %v", err)
	}
	profilename := fmt.Sprintf("profile-%s", o.id)
	if err := client.DeleteProfile(profilename); err != nil {
		o.T.Logf("Failed to delete profile: %v", err)
	}
	networkaddr <- o.network
}

// Command is a command to be run in a node.
type Command struct {
	Node   string
	Line   []string
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

// Run runs a command in a node.
func Run(ctx context.Context, t *testing.T, cmd Command) error {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		t.Fatalf("Failed to connect to LXD: %v", err)
	}
	req := api.InstanceExecPost{
		Command:     cmd.Line,
		WaitForWS:   true,
		Interactive: false,
	}
	done := make(chan bool)
	args := lxd.InstanceExecArgs{
		Stdin:    os.Stdin,
		Stdout:   cmd.Stdout,
		Stderr:   cmd.Stderr,
		DataDone: done,
	}
	op, err := client.ExecInstance(cmd.Node, req, &args)
	if err != nil {
		return err
	}
	if err := op.WaitContext(ctx); err != nil {
		<-done
		return err
	}
	<-done
	if result, ok := op.Get().Metadata["return"]; !ok {
		return fmt.Errorf("no return code")
	} else if result != float64(0) {
		return fmt.Errorf("non-zero return code: %.0f", result)
	}
	return nil
}

// NewTestCluster creates a new cluster and returns an object of type Output
// that can be used to get the created nodes and destroy the cluster when it
// is no longer needed.
func NewTestCluster(in *Input) *Output {
	in.id = uuid.New().String()[:5]
	in.network = <-networkaddr
	PullImage(in)
	CreateProfile(in)
	CreateNetworks(in)
	nodes := CreateNodes(in)
	for _, node := range nodes {
		CopyFilesToNode(in, node)
	}
	return &Output{
		T:       in.T,
		Nodes:   nodes,
		network: in.network,
		id:      in.id,
	}
}

// CopyFilesToNode copies the files needed for the cluster to the node. Copies
// the provided ssh key, the embedded-cluster binary and also all scripts from the
// scripts directory (they are all placed under /usr/local/bin inside the node).
func CopyFilesToNode(in *Input, node string) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	req := lxd.ContainerFileArgs{Mode: 0700, Type: "directory"}
	if err = client.CreateContainerFile(node, "/root/.ssh", req); err != nil {
		in.T.Fatalf("Failed to create directory: %v", err)
	}
	files := []File{
		{
			SourcePath: in.SSHPrivateKey,
			DestPath:   "/root/.ssh/id_rsa",
			Mode:       0600,
		},
		{
			SourcePath: in.SSHPublicKey,
			DestPath:   "/root/.ssh/authorized_keys",
			Mode:       0600,
		},
		{
			SourcePath: in.HelmVMPath,
			DestPath:   "/usr/local/bin/embedded-cluster",
			Mode:       0755,
		},
	}
	scriptFiles, err := scripts.FS.ReadDir(".")
	if err != nil {
		in.T.Fatalf("Failed to read scripts directory: %v", err)
	}
	for _, script := range scriptFiles {
		fp, err := scripts.FS.Open(script.Name())
		if err != nil {
			in.T.Fatalf("Failed to open script %s: %v", script.Name(), err)
		}
		tmp, err := os.CreateTemp("/tmp", fmt.Sprintf("%s-XXXXX.sh", script.Name()))
		if err != nil {
			in.T.Fatalf("Failed to create temporary file: %v", err)
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, fp); err != nil {
			in.T.Fatalf("Failed to copy script %s: %v", script.Name(), err)
		}
		fp.Close()
		files = append(files, File{
			SourcePath: tmp.Name(),
			DestPath:   fmt.Sprintf("/usr/local/bin/%s", script.Name()),
			Mode:       0755,
		})
	}
	for _, file := range files {
		CopyFileToNode(in, node, file)
	}
}

// CopyFileToNode copies a single file to a node.
func CopyFileToNode(in *Input, node string, file File) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	fp, err := os.Open(file.SourcePath)
	if err != nil {
		in.T.Fatalf("Failed to open file %s: %v", file.SourcePath, err)
	}
	defer fp.Close()
	req := lxd.ContainerFileArgs{
		Content: fp,
		Mode:    file.Mode,
		Type:    "file",
	}
	if err := client.CreateContainerFile(node, file.DestPath, req); err != nil {
		in.T.Fatalf("Failed to copy file %s: %v", file.SourcePath, err)
	}
}

// CreateNodes creats the nodes for the cluster. The amount of nodes is
// specified in the input.
func CreateNodes(in *Input) []string {
	nodes := []string{}
	for i := 0; i < in.Nodes; i++ {
		node := CreateNode(in, i)
		NodeHasInternet(in, node)
		nodes = append(nodes, node)
	}
	return nodes
}

// NodeHasInternet checks if the node has internet access. It does this by
// pinging google.com.
func NodeHasInternet(in *Input, node string) {
	in.T.Logf("Testing if node %s can reach the internet", node)
	fp, err := os.CreateTemp("/tmp", "internet-XXXXX.sh")
	if err != nil {
		in.T.Fatalf("Failed to create temporary file: %v", err)
	}
	fp.Close()
	defer func() {
		os.RemoveAll(fp.Name())
	}()
	if err := os.WriteFile(fp.Name(), []byte(checkInternet), 0755); err != nil {
		in.T.Fatalf("Failed to write script: %v", err)
	}
	file := File{
		SourcePath: fp.Name(),
		DestPath:   "/usr/local/bin/check_internet.sh",
		Mode:       0755,
	}
	CopyFileToNode(in, node, file)
	cmd := Command{
		Node:   node,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Line:   []string{"/usr/local/bin/check_internet.sh"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx, in.T, cmd); err != nil {
		in.T.Fatalf("Unable to reach internet from %s: %v", node, err)
	}
}

// CreateNode creates a single node. The i here is used to create a unique
// name for the node. Node is named as "node-<cluster id>-<i>". The node
// name is returned.
func CreateNode(in *Input, i int) string {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	name := fmt.Sprintf("node-%s-%02d", in.id, i)
	profile := fmt.Sprintf("profile-%s", in.id)
	net := fmt.Sprintf("internal-%s", in.id)
	request := api.InstancesPost{
		Name: name,
		Type: api.InstanceTypeContainer,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: in.Image,
		},
		InstancePut: api.InstancePut{
			Profiles:     []string{profile},
			Architecture: "x86_64",
			Config: map[string]string{
				"security.privileged": "true",
			},
			Devices: map[string]map[string]string{
				"eth0": {
					"name":    "eth0",
					"network": net,
					"type":    "nic",
				},
				"kmsg": {
					"path":   "/dev/kmsg",
					"source": "/dev/kmsg",
					"type":   "unix-char",
				},
			},
			Ephemeral: true,
		},
	}
	in.T.Logf("Creating node %s", name)
	if op, err := client.CreateInstance(request); err != nil {
		in.T.Fatalf("Failed to create node %s: %v", name, err)
	} else if err := op.Wait(); err != nil {
		in.T.Fatalf("Failed to wait for node %s: %v", name, err)
	}
	in.T.Logf("Starting node %s", name)
	reqstate := api.InstanceStatePut{Action: "start", Timeout: -1}
	if op, err := client.UpdateInstanceState(name, reqstate, ""); err != nil {
		in.T.Fatalf("Failed to start node %s: %v", name, err)
	} else if err := op.Wait(); err != nil {
		in.T.Fatalf("Failed to wait for node start %s: %v", name, err)
	}
	state := &api.InstanceState{}
	for state.Status != "Running" {
		time.Sleep(5 * time.Second)
		in.T.Logf("Waiting for node %s to start (running)", name)
		if state, _, err = client.GetInstanceState(name); err != nil {
			in.T.Fatalf("Failed to get node state %s: %v", name, err)
		}
	}
	return name
}

// CreateNetworks create two networks, one of type bridge and inside of it another one of
// type ovn, the latter is completely isolated from the host network and from the other
// networks on the same server.
func CreateNetworks(in *Input) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	request := api.NetworksPost{
		Name: fmt.Sprintf("external-%s", in.id),
		Type: "bridge",
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"ipv4.address":     fmt.Sprintf("%s.1/24", in.network),
				"ipv4.dhcp":        "true",
				"ipv4.dhcp.ranges": fmt.Sprintf("%[1]s.2-%[1]s.254", in.network),
				"ipv4.nat":         "true",
				"ipv4.ovn.ranges":  fmt.Sprintf("%[1]s.100-%[1]s.253", in.network),
			},
		},
	}
	if err := client.CreateNetwork(request); err != nil {
		in.T.Fatalf("Failed to create external network: %v", err)
	}
	request = api.NetworksPost{
		Name: fmt.Sprintf("internal-%s", in.id),
		Type: "ovn",
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mtu":   "1500",
				"ipv4.address": "10.0.0.1/24",
				"ipv4.nat":     "true",
				"network":      fmt.Sprintf("external-%s", in.id),
			},
		},
	}
	if err := client.CreateNetwork(request); err != nil {
		in.T.Fatalf("Failed to create internal network: %v", err)
	}
}

// CreateProfile that restricts the hardware and provides privileged access to the
// containers.
func CreateProfile(in *Input) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	request := api.ProfilesPost{
		Name: fmt.Sprintf("profile-%s", in.id),
		ProfilePut: api.ProfilePut{
			Description: fmt.Sprintf("HelmVM test cluster (%s)", in.id),
			Config: map[string]string{
				"raw.lxc": profileConfig,
			},
			Devices: map[string]map[string]string{
				"eth0": {
					"name":    "eth0",
					"network": "lxdbr0",
					"type":    "nic",
				},
				"root": {
					"path": "/",
					"pool": "default",
					"type": "disk",
				},
			},
		},
	}
	if err := client.CreateProfile(request); err != nil {
		in.T.Fatalf("Failed to create profile: %v", err)
	}
}

// PullImage pull the image used for the nodes.
func PullImage(in *Input) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	remote, err := lxd.ConnectSimpleStreams("https://images.linuxcontainers.org", nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to image server: %v", err)
	}
	alias, _, err := remote.GetImageAlias(in.Image)
	if err != nil {
		in.T.Fatalf("Failed to get image alias: %v", err)
	}
	image, _, err := remote.GetImage(alias.Target)
	if err != nil {
		in.T.Fatalf("Failed to get image: %v", err)
	}
	op, err := client.CopyImage(remote, *image, &lxd.ImageCopyArgs{CopyAliases: true})
	if err != nil {
		in.T.Fatalf("Failed to copy image: %v", err)
	}
	if err := op.Wait(); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			in.T.Fatalf("Failed to wait for image copy: %v", err)
		}
	}
}
