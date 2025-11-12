package dryrun

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg-new/replicatedapi"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/firewalld"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/kotsadm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/mock"
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
	Kotsadm                  *Kotsadm
	ReplicatedAPIClient      *ReplicatedAPIClient
	NetworkInterfaceProvider netutils.NetworkInterfaceProvider
	ChooseHostInterfaceImpl  *ChooseInterfaceImpl
	HelmClient               helm.Client
	PreflightRunner          preflights.PreflightRunnerInterface
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
		client.K0sClient = &K0s{
			k0s: new(k0s.K0s),
		}
	}
	if client.Kotsadm == nil {
		client.Kotsadm = NewKotsadm()
	}
	if client.ReplicatedAPIClient != nil {
		replicatedapi.SetClientFactory(func(replicatedAppURL string, license *kotsv1beta1.License, releaseData *release.ReleaseData, opts ...replicatedapi.ClientOption) (replicatedapi.Client, error) {
			return client.ReplicatedAPIClient, nil
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
	k0s.SetClientFactory(func() k0s.K0sInterface {
		return &K0s{
			k0s: new(k0s.K0s),
		}
	})
	kotsadm.Set(client.Kotsadm)

	if client.HelmClient != nil {
		_helmClient = client.HelmClient
	}
	if client.HelmClient != nil {
		helm.SetClientFactory(func(opts helm.HelmOptions) (helm.Client, error) {
			return GetHelmClient(), nil
		})
	}

	if client.PreflightRunner != nil {
		_preflightRunner = client.PreflightRunner
	}
	preflights.Set(_preflightRunner)

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
	data, err := os.ReadFile(drFile)
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

func RecordAppPreflightSpec(apf *troubleshootv1beta2.PreflightSpec) {
	mu.Lock()
	defer mu.Unlock()

	dr.AppPreflightSpec = apf
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

var _helmClient helm.Client

func GetHelmClient() helm.Client {
	if _helmClient == nil {
		_helmClient = &helm.MockClient{}
	}
	return _helmClient
}

var _preflightRunner preflights.PreflightRunnerInterface

func GetPreflightRunner() preflights.PreflightRunnerInterface {
	if _preflightRunner == nil {
		preflightRunner := &preflights.MockPreflightRunner{}
		preflightRunner.
			On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
			Once().
			Run(func(args mock.Arguments) {
				RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
			}).
			Return(&apitypes.PreflightsOutput{
				Pass: []apitypes.PreflightsRecord{
					{
						Title:   "Test Check",
						Message: "Test check passed",
					},
				},
			}, "", nil)
		preflightRunner.
			On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
			Once().
			Run(func(args mock.Arguments) {
				RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
			}).
			Return(&apitypes.PreflightsOutput{
				Pass: []apitypes.PreflightsRecord{
					{
						Title:   "Test Check",
						Message: "Test check passed",
					},
				},
			}, "", nil)
		_preflightRunner = preflightRunner
	}
	return _preflightRunner
}

func Enabled() bool {
	return dr != nil
}
