package console

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
)

type Controller interface {
	ListAvailableNetworkInterfaces() ([]string, error)
}

var _ Controller = (*ConsoleController)(nil)

type ConsoleController struct {
	netUtils utils.NetUtils
}

type ConsoleControllerOption func(*ConsoleController)

func WithNetUtils(netUtils utils.NetUtils) ConsoleControllerOption {
	return func(c *ConsoleController) {
		c.netUtils = netUtils
	}
}

func NewConsoleController(opts ...ConsoleControllerOption) (*ConsoleController, error) {
	controller := &ConsoleController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.netUtils == nil {
		controller.netUtils = utils.NewNetUtils()
	}

	return controller, nil
}

func (c *ConsoleController) ListAvailableNetworkInterfaces() ([]string, error) {
	return c.netUtils.ListValidNetworkInterfaces()
}
