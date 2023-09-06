package config

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/log"

	"github.com/replicatedhq/helmvm/pkg/infra"
	"github.com/replicatedhq/helmvm/pkg/progressbar"
)

// hostcfg is a helper struct for collecting a node's configuration.
type hostcfg struct {
	Address string
	Role    string
	Port    int
	User    string
	KeyPath string
}

// render returns a cluster.Host from the given config.
func (h *hostcfg) render() *cluster.Host {
	ifls := []string{"--force", "--disable-components konnectivity-server"}
	if h.Role == "worker" {
		ifls = []string{"--force"}
	}
	return &cluster.Host{
		Role:         h.Role,
		UploadBinary: true,
		NoTaints:     h.Role == "controller+worker",
		InstallFlags: ifls,
		Connection: rig.Connection{
			SSH: &rig.SSH{
				Address: h.Address,
				Port:    h.Port,
				User:    h.User,
				KeyPath: &h.KeyPath,
			},
		},
	}
}

// testConnection attempts to connect to the host via SSH.
func (h *hostcfg) testConnection() error {
	logger, end := progressbar.Start()
	orig := log.Log
	defer func() {
		close(logger)
		<-end
		log.Log = orig
	}()
	rig.SetLogger(logger)
	return h.render().Connection.Connect()
}

// HostConfigFromInfraNode returns a new hostcfg object with properties copied
// from the given infra.Node.
func hostConfigFromInfraNode(node infra.Node) hostcfg {
	return hostcfg{
		Address: node.Address,
		Role:    node.Role,
		Port:    node.Port,
		User:    node.User,
		KeyPath: node.KeyPath,
	}
}
