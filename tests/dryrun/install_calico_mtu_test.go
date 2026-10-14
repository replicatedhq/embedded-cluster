package dryrun

import (
	"net"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCalicoMTUFromHostInterface verifies that the calico overlay MTU is lowered to fit the MTU of
// the host interface, so pod traffic between nodes is not fragmented on hosts with an MTU below
// 1500.
func TestCalicoMTUFromHostInterface(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	ip := net.ParseIP("10.0.0.11")
	ifaceProvider := &dryrun.NetworkInterfaceProvider{
		Ifaces: []netutils.NetworkInterface{
			&dryrun.NetworkInterface{
				MockName:  "eth1",
				MockFlags: net.FlagUp,
				MockAddrs: []net.Addr{
					&net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)},
				},
				MockMTU: 1450,
			},
		},
	}

	dryrunInstall(t,
		&dryrun.Client{
			HelmClient:               hcli,
			NetworkInterfaceProvider: ifaceProvider,
			ChooseHostInterfaceImpl:  &dryrun.ChooseInterfaceImpl{IP: ip},
		},
	)

	k0sConfig := readK0sConfig(t)
	require.NotNil(t, k0sConfig.Spec.Network.Calico)
	assert.Equal(t, 1400, k0sConfig.Spec.Network.Calico.MTU, "calico mtu should leave room for the vxlan overhead")
}
