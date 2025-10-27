package dryrun

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/firewalld"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	dr     *types.DryRun
	drFile string
	mu     sync.Mutex
)

type Client struct {
	KubeUtils                *KubeUtils
	Helpers                  *Helpers
	Systemd                  *Systemd
	FirewalldUtil            *FirewalldUtil
	Metrics                  *Sender
	K0sClient                *K0s
	HelmClient               helm.Client
	Kotsadm                  *Kotsadm
	NetworkInterfaceProvider netutils.NetworkInterfaceProvider
	ChooseHostInterfaceImpl  *ChooseInterfaceImpl
}

func Init(outputFile string, client *Client) {
	dr = &types.DryRun{
		Flags:             map[string]interface{}{},
		Commands:          []types.Command{},
		Metrics:           []types.Metric{},
		HostPreflightSpec: &troubleshootv1beta2.HostPreflightSpec{},
		LogBuffer:         bytes.NewBuffer(nil),
	}
	drFile = outputFile
	if client == nil {
		client = &Client{}
	}
	if client.KubeUtils == nil {
		client.KubeUtils = &KubeUtils{}
	}
	if client.Helpers == nil {
		client.Helpers = &Helpers{}
	}
	if client.Metrics == nil {
		client.Metrics = &Sender{}
	}
	if client.K0sClient == nil {
		client.K0sClient = &K0s{}
	}
	if client.Kotsadm == nil {
		client.Kotsadm = NewKotsadm()
	}
	if client.HelmClient != nil {
		helm.SetClientFactory(func(opts helm.HelmOptions) (helm.Client, error) {
			return client.HelmClient, nil
		})
	}
	if client.NetworkInterfaceProvider != nil {
		config.NetworkInterfaceProvider = client.NetworkInterfaceProvider
		netutils.DefaultNetworkInterfaceProvider = client.NetworkInterfaceProvider
		config.ChooseHostInterface = client.ChooseHostInterfaceImpl.ChooseHostInterface
	}
	kubeutils.Set(client.KubeUtils)
	helpers.Set(client.Helpers)
	systemd.Set(client.Systemd)
	firewalld.SetUtil(client.FirewalldUtil)
	metrics.Set(client.Metrics)
	k0s.Set(client.K0sClient)
	kotsadm.Set(client.Kotsadm)

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(dr.LogBuffer)
}

func Dump() error {
	mu.Lock()
	defer mu.Unlock()

	dr.LogOutput = dr.LogBuffer.String()
	dr.LogBuffer.Reset()

	output, err := yaml.Marshal(dr)
	if err != nil {
		return fmt.Errorf("marshal dry run info: %w", err)
	}
	if err := os.WriteFile(drFile, output, 0644); err != nil {
		return fmt.Errorf("write dry run info to file: %w", err)
	}
	return nil
}

func Load() (*types.DryRun, error) {
	data, err := helpers.ReadFile(drFile)
	if err != nil {
		return nil, fmt.Errorf("read dry run file: %w", err)
	}

	dr := &types.DryRun{}
	if err := yaml.Unmarshal(data, dr); err != nil {
		return nil, fmt.Errorf("unmarshal dry run file: %w", err)
	}
	return dr, nil
}

func RecordFlags(flagSet *pflag.FlagSet) {
	mu.Lock()
	defer mu.Unlock()

	flagSet.VisitAll(func(flag *pflag.Flag) {
		// Store the flag name and its value
		dr.Flags[flag.Name] = flag.Value.String()
	})
}

func RecordCommand(cmd string, args []string, env map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	fullCmd := cmd
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	dr.Commands = append(dr.Commands, types.Command{
		Cmd: fullCmd,
		Env: env,
	})
}

func RecordMetric(title string, url string, payload []byte) {
	mu.Lock()
	defer mu.Unlock()

	dr.Metrics = append(dr.Metrics, types.Metric{
		Title:   title,
		URL:     url,
		Payload: string(payload),
	})
}

func RecordHostPreflightSpec(hpf *troubleshootv1beta2.HostPreflightSpec) {
	mu.Lock()
	defer mu.Unlock()

	dr.HostPreflightSpec = hpf
}

func WriteFile(path string, content []byte, mode os.FileMode) error {
	fs := dr.Filesystem()
	return afero.WriteFile(fs, path, content, mode)
}

func ReadFile(path string) ([]byte, error) {
	fs := dr.Filesystem()
	return afero.ReadFile(fs, path)
}

func MoveFile(src, dst string) error {
	fs := dr.Filesystem()
	return fs.Rename(src, dst)
}

func Open(path string) (afero.File, error) {
	fs := dr.Filesystem()
	return fs.Open(path)
}

func OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	fs := dr.Filesystem()
	return fs.OpenFile(path, flag, perm)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	fs := dr.Filesystem()

	// afero.ReadDir returns []os.FileInfo
	infos, err := afero.ReadDir(fs, path)
	if err != nil {
		return nil, err
	}

	// Convert []os.FileInfo to []os.DirEntry
	entries := make([]os.DirEntry, len(infos))
	for i, info := range infos {
		entries[i] = fileInfoToDirEntry(info)
	}
	return entries, nil
}

// fileInfoToDirEntry wraps os.FileInfo to implement os.DirEntry
type fileInfoDirEntry struct {
	info os.FileInfo
}

func fileInfoToDirEntry(info os.FileInfo) os.DirEntry {
	return &fileInfoDirEntry{info: info}
}

func (e *fileInfoDirEntry) Name() string {
	return e.info.Name()
}

func (e *fileInfoDirEntry) IsDir() bool {
	return e.info.IsDir()
}

func (e *fileInfoDirEntry) Type() os.FileMode {
	return e.info.Mode().Type()
}

func (e *fileInfoDirEntry) Info() (os.FileInfo, error) {
	return e.info, nil
}

func Stat(path string) (os.FileInfo, error) {
	fs := dr.Filesystem()
	return fs.Stat(path)
}

func Lstat(path string) (os.FileInfo, error) {
	fs := dr.Filesystem()

	// Check if filesystem supports Lstat
	if lstater, ok := fs.(afero.Lstater); ok {
		info, _, err := lstater.LstatIfPossible(path)
		return info, err
	}

	// Fall back to Stat
	return fs.Stat(path)
}

func MkdirTemp(dir, pattern string) (string, error) {
	fs := dr.Filesystem()
	return afero.TempDir(fs, dir, pattern)
}

func CreateTemp(dir, pattern string) (afero.File, error) {
	fs := dr.Filesystem()
	return afero.TempFile(fs, dir, pattern)
}

func RemoveAll(path string) error {
	fs := dr.Filesystem()
	return fs.RemoveAll(path)
}

func Remove(path string) error {
	fs := dr.Filesystem()
	return fs.Remove(path)
}

func Chmod(path string, mode os.FileMode) error {
	fs := dr.Filesystem()
	return fs.Chmod(path, mode)
}

func MkdirAll(path string, perm os.FileMode) error {
	fs := dr.Filesystem()
	return fs.MkdirAll(path, perm)
}

func Rename(oldpath, newpath string) error {
	fs := dr.Filesystem()
	return fs.Rename(oldpath, newpath)
}

func KubeClient() (client.Client, error) {
	return dr.KubeClient()
}

func MetadataClient() (metadata.Interface, error) {
	return dr.MetadataClient()
}

func GetClientSet() (kubernetes.Interface, error) {
	return dr.GetClientset()
}

func Enabled() bool {
	return dr != nil
}
