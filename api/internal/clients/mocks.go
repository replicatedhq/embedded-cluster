package clients

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// MockRESTClientGetter is a mock implementation of genericclioptions.RESTClientGetter
type MockRESTClientGetter struct {
	mock.Mock
}

func (m *MockRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	args := m.Called()
	return args.Get(0).(*rest.Config), args.Error(1)
}

func (m *MockRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(discovery.CachedDiscoveryInterface), args.Error(1)
}

func (m *MockRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(meta.RESTMapper), args.Error(1)
}

func (m *MockRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(clientcmd.ClientConfig)
}
