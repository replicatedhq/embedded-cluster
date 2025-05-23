package console

import (
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type Controller interface {
	GetBranding() (types.Branding, error)
	ListAvailableNetworkInterfaces() ([]string, error)
}

type ConsoleController struct {
	utils.NetUtils
}

type ConsoleControllerOption func(*ConsoleController)

func WithNetUtils(netUtils utils.NetUtils) ConsoleControllerOption {
	return func(c *ConsoleController) {
		c.NetUtils = netUtils
	}
}

func NewConsoleController(opts ...ConsoleControllerOption) (*ConsoleController, error) {
	controller := &ConsoleController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.NetUtils == nil {
		controller.NetUtils = utils.NewNetUtils()
	}

	return controller, nil
}

func (c *ConsoleController) GetBranding() (types.Branding, error) {
	// TODO
	return types.Branding{
		ApplicationName: "Embedded Cluster",
		LogoURL:         "https://replicated.com/logo.png",
	}, nil
}

func (c *ConsoleController) ListAvailableNetworkInterfaces() ([]string, error) {
	return c.NetUtils.ListValidNetworkInterfaces()
}
