package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/google/uuid"
)

var networkaddr chan string

const lxdSocket = "/var/snap/lxd/common/lxd/unix.socket"
const profileConfig = `lxc.apparmor.profile=unconfined
lxc.mount.auto=proc:rw sys:rw cgroup:rw
lxc.cgroup.devices.allow=a
lxc.cap.drop=`
const checkInternet = `#!/bin/bash
timeout 5 bash -c 'cat < /dev/null > /dev/tcp/api.replicated.com/80'
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

// NoopCloser adds a Close to a bytes.Buffer.
type NoopCloser struct {
	*bytes.Buffer
}

// Close is the no-op of the NoopCloser.
func (n *NoopCloser) Close() error {
	return nil
}

// Input are the options passed in to the cluster creation plus some data
// for internal consumption only.
type Input struct {
	Nodes                             int
	CreateRegularUser                 bool
	LicensePath                       string
	EmbeddedClusterPath               string
	EmbeddedClusterReleaseBuilderPath string // used to replace the release in the binary
	AirgapInstallBundlePath           string
	AirgapUpgradeBundlePath           string
	Image                             string
	network                           string
	T                                 *testing.T
	WithProxy                         bool
	id                                string
}

// File holds information about a file that must be uploaded to a node.
type File struct {
	SourcePath string
	DestPath   string
	Mode       int
}

// Dir holds information about a directory that must be uploaded to a node.
type Dir struct {
	SourcePath string
	DestPath   string
}

// Output is returned when a cluster is created. Contain a list of all node
// names and the cluster id.
type Output struct {
	Nodes   []string
	network string
	id      string
	T       *testing.T
	Proxy   string
}

// Destroy destroys a cluster pointed by the id property inside the output.
func (o *Output) Destroy() {
	o.T.Logf("Destroying cluster %s", o.id)
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		o.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	nodes := o.Nodes
	if o.Proxy != "" {
		nodes = append(nodes, o.Proxy)
	}
	for _, node := range nodes {
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
	Node        string
	Line        []string
	Stdout      io.WriteCloser
	Stderr      io.WriteCloser
	RegularUser bool
	Env         map[string]string
}

// Run runs a command in a node.
func Run(ctx context.Context, t *testing.T, cmd Command) error {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		t.Fatalf("Failed to connect to LXD: %v", err)
	}
	env := map[string]string{}
	var uid uint32
	if cmd.RegularUser {
		uid = 9999
		env["HOME"] = "/home/user"
	}
	for k, v := range cmd.Env {
		env[k] = v
	}
	req := api.InstanceExecPost{
		Command:     cmd.Line,
		WaitForWS:   true,
		Interactive: false,
		User:        uid,
		Environment: env,
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

// imagesMap maps some image names so we can use them in the tests.
// For example, the letter "j" doesn't say it is an ubuntu/jammy.
var imagesMap = map[string]string{
	"ubuntu/jammy": "j",
}

// NewTestCluster creates a new cluster and returns an object of type Output
// that can be used to get the created nodes and destroy the cluster when it
// is no longer needed.
func NewTestCluster(in *Input) *Output {
	if name, ok := imagesMap[in.Image]; ok {
		in.Image = name
	}

	in.id = uuid.New().String()[:5]
	in.network = <-networkaddr

	out := &Output{
		T:       in.T,
		network: in.network,
		id:      in.id,
	}
	out.T.Cleanup(out.Destroy)

	PullImage(in, in.Image)
	if ProxyImage != in.Image {
		PullImage(in, ProxyImage)
	}
	CreateProfile(in)
	CreateNetworks(in)
	out.Nodes = CreateNodes(in)
	for _, node := range out.Nodes {
		CopyFilesToNode(in, node)
		CopyDirsToNode(in, node)
		if in.CreateRegularUser {
			CreateRegularUser(in, node)
		}
	}
	// We create a proxy node for all installations to run playwright tests.
	out.Proxy = CreateProxy(in)
	CopyDirsToNode(in, out.Proxy)
	if in.CreateRegularUser {
		CreateRegularUser(in, out.Proxy)
	}
	NodeHasInternet(in, out.Proxy)
	ConfigureProxyNode(in)
	if in.WithProxy {
		ConfigureProxy(in)
	}
	opts := []RunCommandOption{}
	if in.WithProxy {
		opts = append(opts, WithEnv(map[string]string{
			"http_proxy":  HTTPProxy,
			"https_proxy": HTTPProxy,
		}))
	}
	for _, node := range out.Nodes {
		in.T.Logf("Installing deps on node %s", node)
		RunCommandOnNode(in, []string{"install-deps.sh"}, node, opts...)
	}
	return out
}

const ProxyImage = "debian/12"
const HTTPProxy = "http://10.0.0.254:3128"
const NOProxy = "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"

// CreateProxy creates a node that attaches to both networks (external and internal),
// once this is done we install squid and configure it to be a proxy. We also make
// sure that all nodes are configured to use the proxy as default gateway. Internet
// won't work on them by design (exception made for DNS requests and http requests
// using the proxy). Proxy is accessible from the cluster nodes on 10.0.0.254:3128.
func CreateProxy(in *Input) string {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}
	name := fmt.Sprintf("node-%s-proxy", in.id)
	profile := fmt.Sprintf("profile-%s", in.id)
	innet := fmt.Sprintf("external-%s", in.id)
	exnet := fmt.Sprintf("internal-%s", in.id)
	request := api.InstancesPost{
		Name: name,
		Type: api.InstanceTypeContainer,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: ProxyImage,
		},
		InstancePut: api.InstancePut{
			Profiles:     []string{profile},
			Architecture: "x86_64",
			Devices: map[string]map[string]string{
				"eth0": {
					"name":    "eth0",
					"network": innet,
					"type":    "nic",
				},
				"eth1": {
					"name":    "eth1",
					"network": exnet,
					"type":    "nic",
				},
			},
			Ephemeral: true,
		},
	}
	in.T.Logf("Creating proxy %s", name)
	if op, err := client.CreateInstance(request); err != nil {
		in.T.Fatalf("Failed to create proxy %s: %v", name, err)
	} else if err := op.Wait(); err != nil {
		in.T.Fatalf("Failed to wait for proxy %s: %v", name, err)
	}
	in.T.Logf("Starting proxy %s", name)
	reqstate := api.InstanceStatePut{Action: "start", Timeout: -1}
	if op, err := client.UpdateInstanceState(name, reqstate, ""); err != nil {
		in.T.Fatalf("Failed to start proxy %s: %v", name, err)
	} else if err := op.Wait(); err != nil {
		in.T.Fatalf("Failed to wait for proxy start %s: %v", name, err)
	}
	state := &api.InstanceState{}
	for state.Status != "Running" {
		time.Sleep(5 * time.Second)
		in.T.Logf("Waiting for proxy %s to start (running)", name)
		if state, _, err = client.GetInstanceState(name); err != nil {
			in.T.Fatalf("Failed to get proxy state %s: %v", name, err)
		}
	}
	return name
}

// ConfigureProxyNode installs squid and iptables on the target node. Configures the needed
// ip addresses and sets up iptables to allow nat for requests coming out on eth0 using
// port 53(UDP).
func ConfigureProxyNode(in *Input) {
	proxyName := fmt.Sprintf("node-%s-proxy", in.id)

	// starts by installing dependencies, setting up the second network interface ip
	// address and configuring iptables to allow dns requests forwarding (nat).
	for _, cmd := range [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "-y", "iptables", "squid"},
		{"ip", "addr", "add", "10.0.0.254/24", "dev", "eth1"},
		{"ip", "link", "set", "eth1", "up"},
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
		{"iptables", "-t", "nat", "-o", "eth0", "-A", "POSTROUTING", "-p", "udp", "--dport", "53", "-j", "MASQUERADE"},
	} {
		RunCommandOnNode(in, cmd, proxyName)
	}
}

// ConfigureProxy configures squid to accept requests coming from 10.0.0.0/24 network.
// Proxy will be listening on http://10.0.0.254:3128. It also sets the default route
// on all other nodes to point to the proxy to ensure no internet will work on them
// other than dns and http requests using the proxy.
func ConfigureProxy(in *Input) {
	proxyName := fmt.Sprintf("node-%s-proxy", in.id)

	// create a simple squid configuration that allows for localnet access. upload it
	// to the proxy in the right location. restart squid to apply the configuration.
	tmpfile, err := os.CreateTemp("", "squid-config-*.conf")
	if err != nil {
		in.T.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err = tmpfile.WriteString("http_access allow localnet\n"); err != nil {
		in.T.Fatalf("Failed to write to temp file: %v", err)
	}
	file := File{SourcePath: tmpfile.Name(), DestPath: "/etc/squid/conf.d/ec.conf", Mode: 0644}
	tmpfile.Close()
	CopyFileToNode(in, proxyName, file)
	RunCommandOnNode(in, []string{"systemctl", "restart", "squid"}, proxyName)

	// set the default route on all other nodes to point to the proxy we just created.
	// this makes it easier to ensure no internet will work on them other than dns and
	// http requests using the proxy.
	for i := 0; i < in.Nodes; i++ {
		name := fmt.Sprintf("node-%s-%02d", in.id, i)
		for _, cmd := range [][]string{
			{"ip", "route", "del", "default"},
			{"ip", "route", "add", "default", "via", "10.0.0.254"},
		} {
			RunCommandOnNode(in, cmd, name)
		}
	}
}

type RunCommandOption func(cmd *Command)

func WithEnv(env map[string]string) RunCommandOption {
	return func(cmd *Command) {
		cmd.Env = env
	}
}

// RunCommand runs the provided command on the provided node (name). Implements a
// timeout of 2 minutes for the command to run and if it fails calls T.Failf().
func RunCommandOnNode(in *Input, cmdline []string, name string, opts ...RunCommandOption) {
	in.T.Logf("Running `%s` on node %s", strings.Join(cmdline, " "), name)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := Command{
		Node:   name,
		Line:   cmdline,
		Stdout: &NoopCloser{stdout},
		Stderr: &NoopCloser{stderr},
	}
	for _, fn := range opts {
		fn(&cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := Run(ctx, in.T, cmd)
	in.T.Logf("stdout: %s", stdout.String())
	in.T.Logf("stderr: %s", stderr.String())
	if err != nil {
		in.T.Fatalf("Failed to run command: %v", err)
	}
}

// CreateRegularUser adds an unprivileged user to the node. The username is
// "user" and there is no password. Creates the user with UID 9999.
func CreateRegularUser(in *Input, node string) {
	in.T.Logf("Creating regular user `user(9999)` on node %s", node)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := Command{
		Node:   node,
		Stdout: &NoopCloser{stdout},
		Stderr: &NoopCloser{stderr},
		Line: []string{
			"useradd",
			"-d", "/home/user",
			"-s", "/bin/bash",
			"-u", "9999",
			"-m",
			"user",
		},
	}
	if err := Run(ctx, in.T, cmd); err != nil {
		in.T.Logf("stdout: %s", stdout.String())
		in.T.Logf("stderr: %s", stderr.String())
		in.T.Fatalf("Unable to create regular user: %s", err)
	}
}

// CopyFilesToNode copies the files needed for the cluster to the node. Copies
// the provided ssh key and the embedded-cluster release files.
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
			SourcePath: in.LicensePath,
			DestPath:   "/assets/license.yaml",
			Mode:       0644,
		},
		{
			SourcePath: in.EmbeddedClusterPath,
			DestPath:   "/usr/local/bin/embedded-cluster",
			Mode:       0755,
		},
		{
			SourcePath: in.EmbeddedClusterReleaseBuilderPath,
			DestPath:   "/usr/local/bin/embedded-cluster-release-builder",
			Mode:       0755,
		},
		{
			SourcePath: in.AirgapInstallBundlePath,
			DestPath:   "/assets/ec-release.tgz",
			Mode:       0755,
		},
		{
			SourcePath: in.AirgapUpgradeBundlePath,
			DestPath:   "/assets/ec-release-upgrade.tgz",
			Mode:       0755,
		},
	}
	for _, file := range files {
		CopyFileToNode(in, node, file)
	}
}

// CopyDirsToNode copies the directories needed to the node.
func CopyDirsToNode(in *Input, node string) {
	dirs := []Dir{
		{
			SourcePath: "scripts",
			DestPath:   "/usr/local/bin",
		},
		{
			SourcePath: "playwright",
			DestPath:   "/automation/playwright",
		},
	}
	for _, dir := range dirs {
		CopyDirToNode(in, node, dir)
	}
}

// CopyDirToNode copies a single directory to a node.
func CopyDirToNode(in *Input, node string, dir Dir) {
	if err := filepath.Walk(dir.SourcePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir.SourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}
		file := File{
			SourcePath: path,
			DestPath:   filepath.Join(dir.DestPath, relPath),
			Mode:       int(info.Mode()),
		}
		CopyFileToNode(in, node, file)
		return nil
	}); err != nil {
		in.T.Fatalf("Failed to walk directory %s: %v", dir.SourcePath, err)
	}
}

// CopyFileToNode copies a single file to a node.
func CopyFileToNode(in *Input, node string, file File) {
	if file.SourcePath == "" {
		in.T.Logf("Skipping file %s: source path is empty", file.DestPath)
		return
	}
	// ensure destination path exists
	for _, cmd := range [][]string{
		{"mkdir", "-p", filepath.Dir(file.DestPath)},
	} {
		RunCommandOnNode(in, cmd, node)
	}
	in.T.Logf("Copying `%s` to `%s` on node %s", file.SourcePath, file.DestPath, node)
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
		in.T.Fatalf("Failed to copy file `%s` to `%s` on node %s: %v", file.SourcePath, file.DestPath, node, err)
	}
}

// CopyFileFromNode copies a file from a node to the host.
func CopyFileFromNode(node, source, dest string) error {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to LXD: %v", err)
	}
	content, _, err := client.GetContainerFile(node, source)
	if err != nil {
		return fmt.Errorf("failed to get file %s: %v", source, err)
	}
	fp, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", dest, err)
	}
	defer fp.Close()
	if _, err := io.Copy(fp, content); err != nil {
		return fmt.Errorf("failed to copy file %s: %v", dest, err)
	}
	return nil
}

// CreateNodes creats the nodes for the cluster. The amount of nodes is
// specified in the input.
func CreateNodes(in *Input) []string {
	nodes := []string{}
	for i := 0; i < in.Nodes; i++ {
		node := CreateNode(in, i)
		if !in.WithProxy {
			NodeHasInternet(in, node)
		} else {
			NodeHasNoInternet(in, node)
		}
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
	var success bool
	var lastErr error
	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := Run(ctx, in.T, cmd); err != nil {
			lastErr = fmt.Errorf("failed to check internet: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		success = true
		break
	}
	if !success {
		in.T.Fatalf("Timed out trying to reach internet from %s: %v", node, lastErr)
	}
	in.T.Logf("Node %s can reach the internet", node)
}

// NodeHasNoInternet checks if the node has internet access and fails if so. It does this by
// pinging google.com.
func NodeHasNoInternet(in *Input, node string) {
	in.T.Logf("Ensuring node %s cannot reach the internet", node)
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
	if err := Run(ctx, in.T, cmd); err == nil {
		in.T.Fatalf("Air gap node %s can reach the internet", node)
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
			Devices: map[string]map[string]string{
				"eth0": {
					"name":    "eth0",
					"network": net,
					"type":    "nic",
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
				"ipv4.routes":      "10.0.0.0/24",
			},
		},
	}
	if err := client.CreateNetwork(request); err != nil {
		in.T.Fatalf("Failed to create external network: %v", err)
	}
	open := "true"
	if in.WithProxy {
		open = "false"
	}
	request = api.NetworksPost{
		Name: fmt.Sprintf("internal-%s", in.id),
		Type: "ovn",
		NetworkPut: api.NetworkPut{
			Config: map[string]string{
				"bridge.mtu":   "1500",
				"ipv4.address": "10.0.0.1/24",
				"ipv4.nat":     open,
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
			Description: fmt.Sprintf("Embedded Cluster test cluster (%s)", in.id),
			Config: map[string]string{
				"raw.lxc":              profileConfig,
				"security.nesting":     "true",
				"security.privileged":  "true",
				"linux.kernel_modules": "br_netfilter,ip_tables,ip6_tables,netlink_diag,nf_nat,overlay",
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
				"kmsg": {
					"path":   "/dev/kmsg",
					"source": "/dev/kmsg",
					"type":   "unix-char",
				},
			},
		},
	}
	if err := client.CreateProfile(request); err != nil {
		in.T.Fatalf("Failed to create profile: %v", err)
	}
}

// PullImage pull the image used for the nodes.
func PullImage(in *Input, image string) {
	client, err := lxd.ConnectLXDUnix(lxdSocket, nil)
	if err != nil {
		in.T.Fatalf("Failed to connect to LXD: %v", err)
	}

	for _, server := range []string{
		"https://images.lxd.canonical.com",
		"https://cloud-images.ubuntu.com/minimal/releases",
	} {
		in.T.Logf("Pulling %q image from %s at %s", image, server, time.Now().Format(time.RFC3339))
		remote, err := lxd.ConnectSimpleStreams(server, nil)
		if err != nil {
			in.T.Fatalf("Failed to connect to image server: %v", err)
		}

		alias, _, err := remote.GetImageAlias(image)
		if err != nil {
			in.T.Logf("Failed to get image alias %s on %s: %v", image, server, err)
			continue
		}

		image, _, err := remote.GetImage(alias.Target)
		if err != nil {
			in.T.Logf("Failed to get image %s on %s: %v", alias.Target, server, err)
			continue
		}

		op, err := client.CopyImage(remote, *image, &lxd.ImageCopyArgs{CopyAliases: true})
		if err != nil {
			in.T.Logf("Failed to copy image %s from %s: %v", alias.Target, server, err)
			continue
		}

		if err = op.Wait(); err == nil || strings.Contains(err.Error(), "already exists") {
			return
		}
		in.T.Logf("Failed to wait for image copy: %v", err)
	}

	in.T.Fatalf("Failed to pull image %s (tried in all servers)", image)
}
