package console

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

type Controller interface {
	GetBranding() (types.Branding, error)
	ListAvailableNetworkInterfaces() ([]string, error)
}

var _ Controller = (*ConsoleController)(nil)

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
	app := release.GetApplication()
	if app == nil {
		return types.Branding{}, fmt.Errorf("application not found")
	}

	return types.Branding{
		AppTitle: app.Spec.Title,
		AppIcon:  app.Spec.Icon,
	}, nil
}

func (c *ConsoleController) ListAvailableNetworkInterfaces() ([]string, error) {
	return c.NetUtils.ListValidNetworkInterfaces()
}
