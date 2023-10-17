package config

import (
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/log"
	"github.com/sirupsen/logrus"

	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
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
	var ifls []string
	if h.Role != "worker" {
		ifls = []string{"--disable-components konnectivity-server"}
	}
	return &cluster.Host{
		Role:         h.Role,
		UploadBinary: false,
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
	mask := func(raw string) string {
		logrus.StandardLogger().Writer().Write([]byte(raw))
		return fmt.Sprintf("Validating host %s", h.Address)
	}
	loading := pb.Start(pb.WithMask(mask))
	orig := log.Log
	defer func() {
		loading.Close()
		log.Log = orig
	}()
	rig.SetLogger(loading)
	return h.render().Connection.Connect()
}
