// Package ec2 provisions single-node clusters on EC2. It exists for tests that
// need a real Red Hat Enterprise Linux host, which the Compatibility Matrix
// does not offer -- selinux confinement depends on container-selinux, and
// container-selinux only ships on RHEL-family distros.
//
// Deliberately smaller than the cmx backend: online installs only. Airgap
// emulation is what makes a cluster backend expensive to write, and it has
// nothing to do with selinux.
package ec2

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// rhelOwnerID is Red Hat's AWS account. Filtering on it avoids picking up a
	// community image that merely has "RHEL" in the name.
	rhelOwnerID = "309956199498"

	// tagKey marks everything this backend creates. The CI credentials are only
	// permitted to delete resources carrying it, so a bug here cannot terminate
	// anything else in the account.
	tagKey = "ec-e2e-selinux"
)

type ClusterInput struct {
	T *testing.T

	// RHELVersionPrefix selects the AMI, e.g. "9" or "9.6". Defaults to "9".
	RHELVersionPrefix string
	// InstanceType defaults to instanceTypeDefault.
	InstanceType string
	// Region defaults to $AWS_REGION, then regionDefault.
	Region string
}

const (
	instanceTypeDefault = "m6i.xlarge"
	regionDefault       = "us-east-1"
	diskSizeGiB         = 100

	// The nodeport bypass-kurl-proxy.sh creates, pointing straight at kotsadm.
	// Going through kurl-proxy would mean handling its self-signed certificate
	// in the browser.
	adminConsoleNodePort = "30003"
)

type Cluster struct {
	Nodes []Node

	t           *testing.T
	region      string
	keyName     string
	keyPath     string
	sgID        string
	cleanupOnce sync.Once
}

type Node struct {
	ID        string
	PublicIP  string
	PrivateIP string
}

// NewCluster provisions one RHEL node and waits for ssh. It registers its own
// cleanup with t, so a panicking or failing test still tears the node down.
func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{
		t:      in.T,
		region: firstNonEmpty(in.Region, os.Getenv("AWS_REGION"), regionDefault),
	}
	c.t.Cleanup(func() { c.Cleanup() })

	if err := c.provision(in); err != nil {
		// Cleanup still runs via t.Cleanup, so partial state is not leaked.
		in.T.Fatalf("failed to provision ec2 cluster: %v", err)
	}

	return c
}

func (c *Cluster) provision(in *ClusterInput) error {
	suffix := randomSuffix()

	ami, err := c.latestRHELAMI(firstNonEmpty(in.RHELVersionPrefix, "9"))
	if err != nil {
		return fmt.Errorf("find rhel ami: %w", err)
	}
	c.t.Logf("%s: using rhel ami %s", time.Now().Format(time.RFC3339), ami)

	if err := c.createKeyPair(suffix); err != nil {
		return fmt.Errorf("create key pair: %w", err)
	}

	if err := c.createSecurityGroup(suffix); err != nil {
		return fmt.Errorf("create security group: %w", err)
	}

	node, err := c.runInstance(ami, firstNonEmpty(in.InstanceType, instanceTypeDefault))
	if err != nil {
		return fmt.Errorf("run instance: %w", err)
	}
	c.Nodes = append(c.Nodes, node)
	c.t.Logf("%s: instance %s running at %s", time.Now().Format(time.RFC3339), node.ID, node.PublicIP)

	if err := c.waitForSSH(node); err != nil {
		return fmt.Errorf("wait for ssh: %w", err)
	}

	return nil
}

func (c *Cluster) latestRHELAMI(versionPrefix string) (string, error) {
	out, err := c.aws(
		"ec2", "describe-images",
		"--owners", rhelOwnerID,
		"--filters",
		fmt.Sprintf("Name=name,Values=RHEL-%s*_HVM-*-x86_64-*Hourly2*", versionPrefix),
		"Name=state,Values=available",
		"--query", "reverse(sort_by(Images,&CreationDate))[0].ImageId",
		"--output", "text",
	)
	if err != nil {
		return "", err
	}
	ami := strings.TrimSpace(out)
	if ami == "" || ami == "None" {
		return "", fmt.Errorf("no rhel %s ami found", versionPrefix)
	}
	return ami, nil
}

func (c *Cluster) createKeyPair(suffix string) error {
	c.keyName = fmt.Sprintf("%s-%s", tagKey, suffix)
	c.keyPath = fmt.Sprintf("%s/%s.pem", c.t.TempDir(), c.keyName)

	// Generate locally and upload only the public half, so aws never holds a
	// private key. Matches what the cmx setup action does.
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", c.keyName, "-f", c.keyPath).CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-keygen: %w: %s", err, string(out))
	}

	if _, err := c.aws(
		"ec2", "import-key-pair",
		"--key-name", c.keyName,
		"--public-key-material", "fileb://"+c.keyPath+".pub",
		"--tag-specifications", c.tagSpec("key-pair"),
	); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) createSecurityGroup(suffix string) error {
	out, err := c.aws(
		"ec2", "create-security-group",
		"--group-name", fmt.Sprintf("%s-%s", tagKey, suffix),
		"--description", "embedded cluster selinux e2e",
		"--tag-specifications", c.tagSpec("security-group"),
		"--query", "GroupId", "--output", "text",
	)
	if err != nil {
		return err
	}
	c.sgID = strings.TrimSpace(out)

	// ssh for the test itself, and the nodeport bypass-kurl-proxy.sh creates so
	// playwright can reach the admin console from the runner.
	for _, port := range []string{"22", adminConsoleNodePort} {
		if _, err := c.aws(
			"ec2", "authorize-security-group-ingress",
			"--group-id", c.sgID,
			"--protocol", "tcp", "--port", port, "--cidr", "0.0.0.0/0",
		); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) runInstance(ami, instanceType string) (Node, error) {
	out, err := c.aws(
		"ec2", "run-instances",
		"--image-id", ami,
		"--instance-type", instanceType,
		"--count", "1",
		"--key-name", c.keyName,
		"--security-group-ids", c.sgID,
		"--block-device-mappings", fmt.Sprintf(
			`[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":%d,"VolumeType":"gp3","DeleteOnTermination":true}}]`, diskSizeGiB),
		"--tag-specifications", c.tagSpec("instance"), c.tagSpec("volume"),
		"--query", "Instances[0].InstanceId", "--output", "text",
	)
	if err != nil {
		return Node{}, err
	}
	id := strings.TrimSpace(out)

	if _, err := c.aws("ec2", "wait", "instance-running", "--instance-ids", id); err != nil {
		return Node{}, fmt.Errorf("wait instance-running: %w", err)
	}

	out, err = c.aws(
		"ec2", "describe-instances", "--instance-ids", id,
		"--query", "Reservations[0].Instances[0].[PublicIpAddress,PrivateIpAddress]",
		"--output", "text",
	)
	if err != nil {
		return Node{}, err
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return Node{}, fmt.Errorf("unexpected describe-instances output: %q", out)
	}

	return Node{ID: id, PublicIP: fields[0], PrivateIP: fields[1]}, nil
}

// tagSpec builds a --tag-specifications value. Alongside our own tagKey it sets
// the account-wide convention (expires-on, owner, Name) so the organization's
// reaper can also collect anything we leak.
func (c *Cluster) tagSpec(resourceType string) string {
	return fmt.Sprintf(
		"ResourceType=%s,Tags=["+
			"{Key=%s,Value=true},"+
			"{Key=Name,Value=ec-selinux-e2e},"+
			"{Key=owner,Value=embedded-cluster-ci},"+
			"{Key=expires-on,Value=%s}]",
		resourceType, tagKey, time.Now().UTC().Format("2006-01-02"),
	)
}

func (c *Cluster) waitForSSH(node Node) error {
	c.t.Logf("%s: waiting for ssh on %s", time.Now().Format(time.RFC3339), node.PublicIP)
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(5 * time.Second)
	var lastErr error

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out after 5 minutes: last error: %w", lastErr)
		case <-tick:
			_, _, err := c.RunCommandOnNode(0, []string{"uptime"})
			if err == nil {
				c.t.Logf("%s: ssh is up on %s", time.Now().Format(time.RFC3339), node.PublicIP)
				return nil
			}
			lastErr = err
		}
	}
}

// RunCommandOnNode runs line on the node over ssh, as root. RHEL's cloud images
// use ec2-user, which has passwordless sudo.
func (c *Cluster) RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error) {
	if node >= len(c.Nodes) {
		return "", "", fmt.Errorf("node %d does not exist", node)
	}

	for _, env := range envs {
		for k, v := range env {
			line = append([]string{fmt.Sprintf("%s=%s", k, v)}, line...)
		}
	}
	// Via env, not a bare `sudo PATH=...`: sudo resolves the command against
	// secure_path, which on RHEL is only /sbin:/bin:/usr/sbin:/usr/bin. Both
	// /usr/local/bin and the data dir fall outside it, so a command-line PATH
	// assignment sets the variable but the lookup still fails. env does the
	// lookup itself, using the PATH we give it.
	line = append([]string{"sudo", "env", "PATH=$PATH:/usr/local/bin:/var/lib/embedded-cluster/bin"}, line...)

	args := []string{
		"-i", c.keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("ec2-user@%s", c.Nodes[node].PublicIP),
		strings.Join(line, " "),
	}

	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), stderr.String(), nil
}

// CopyFileToNode uploads a local file, used to get the embedded-cluster binary
// onto the host.
func (c *Cluster) CopyFileToNode(node int, src, dst string) error {
	if node >= len(c.Nodes) {
		return fmt.Errorf("node %d does not exist", node)
	}

	tmp := "/tmp/" + strings.TrimPrefix(dst[strings.LastIndex(dst, "/")+1:], "/")
	args := []string{
		"-i", c.keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		src,
		fmt.Sprintf("ec2-user@%s:%s", c.Nodes[node].PublicIP, tmp),
	}
	if out, err := exec.Command("scp", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("scp %s: %w: %s", src, err, string(out))
	}

	if stdout, stderr, err := c.RunCommandOnNode(node, []string{"install", "-m", "0755", tmp, dst}); err != nil {
		return fmt.Errorf("install %s: %w: %s: %s", dst, err, stdout, stderr)
	}
	return nil
}

// Cleanup terminates the instance and removes the security group and key pair.
// Safe to call more than once.
func (c *Cluster) Cleanup(envs ...map[string]string) {
	c.cleanupOnce.Do(func() {
		for _, node := range c.Nodes {
			c.t.Logf("%s: terminating instance %s", time.Now().Format(time.RFC3339), node.ID)
			if _, err := c.aws("ec2", "terminate-instances", "--instance-ids", node.ID); err != nil {
				c.t.Logf("failed to terminate instance %s: %v", node.ID, err)
			}
		}

		// The security group cannot be deleted while the instance still holds a
		// network interface, so wait for termination first.
		for _, node := range c.Nodes {
			if _, err := c.aws("ec2", "wait", "instance-terminated", "--instance-ids", node.ID); err != nil {
				c.t.Logf("failed waiting for instance %s to terminate: %v", node.ID, err)
			}
		}

		if c.sgID != "" {
			if _, err := c.aws("ec2", "delete-security-group", "--group-id", c.sgID); err != nil {
				c.t.Logf("failed to delete security group %s: %v", c.sgID, err)
			}
		}
		if c.keyName != "" {
			if _, err := c.aws("ec2", "delete-key-pair", "--key-name", c.keyName); err != nil {
				c.t.Logf("failed to delete key pair %s: %v", c.keyName, err)
			}
		}
	})
}

func (c *Cluster) SetupPlaywrightAndRunTest(testName string, args ...string) (string, string, error) {
	if err := c.SetupPlaywright(); err != nil {
		return "", "", fmt.Errorf("setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(testName, args...)
}

// SetupPlaywright publishes the admin console on a nodeport and installs
// playwright on the runner. Playwright itself runs next to the test, not on the
// node, so all the node needs is a reachable console.
func (c *Cluster) SetupPlaywright(envs ...map[string]string) error {
	if err := c.bypassKurlProxy(envs...); err != nil {
		return err
	}
	return c.npmInstallPlaywright()
}

// bypassKurlProxy points a nodeport straight at kotsadm, skipping kurl-proxy
// and its self-signed certificate. The scripts are not on a RHEL cloud image
// the way they are baked into cmx images, so copy them over first.
func (c *Cluster) bypassKurlProxy(envs ...map[string]string) error {
	c.t.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	for _, script := range []string{"common.sh", "bypass-kurl-proxy.sh"} {
		if err := c.CopyFileToNode(0, "scripts/"+script, "/usr/local/bin/"+script); err != nil {
			return fmt.Errorf("copy %s: %w", script, err)
		}
	}
	if stdout, stderr, err := c.RunCommandOnNode(0, []string{"/usr/local/bin/bypass-kurl-proxy.sh"}, envs...); err != nil {
		return fmt.Errorf("bypass kurl-proxy: %w: %s: %s", err, stdout, stderr)
	}
	return nil
}

func (c *Cluster) npmInstallPlaywright() error {
	c.t.Logf("%s: installing playwright", time.Now().Format(time.RFC3339))
	out, err := exec.Command("sh", "-c", "cd playwright && npm ci && npx playwright install --with-deps").CombinedOutput()
	if err != nil {
		return fmt.Errorf("install playwright: %w: %s", err, string(out))
	}
	return nil
}

func (c *Cluster) RunPlaywrightTest(testName string, args ...string) (string, string, error) {
	c.t.Logf("%s: running playwright test %s", time.Now().Format(time.RFC3339), testName)
	cmd := exec.Command("scripts/playwright.sh", append([]string{testName}, args...)...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BASE_URL=http://%s:%s", c.Nodes[0].PublicIP, adminConsoleNodePort),
		"PLAYWRIGHT_DIR=./playwright",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func (c *Cluster) aws(args ...string) (string, error) {
	args = append(args, "--region", c.region)
	cmd := exec.Command("aws", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("aws %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// only used to keep concurrent runs from colliding on resource names
		return fmt.Sprintf("%d", time.Now().UnixNano()%1e8)
	}
	return fmt.Sprintf("%x", b)
}
