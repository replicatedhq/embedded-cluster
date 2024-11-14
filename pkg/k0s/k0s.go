package k0s

import (
	"context"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

var (
	_k0s ClientInterface
)

func init() {
	Set(&Client{})
}

func Set(k0s ClientInterface) {
	_k0s = k0s
}

type K0sStatus struct {
	Role          string                   `json:"Role"`
	Vars          K0sVars                  `json:"K0sVars"`
	ClusterConfig k0sv1beta1.ClusterConfig `json:"ClusterConfig"`
}

type K0sVars struct {
	AdminKubeConfigPath   string `json:"AdminKubeConfigPath"`
	KubeletAuthConfigPath string `json:"KubeletAuthConfigPath"`
	CertRootDir           string `json:"CertRootDir"`
	EtcdCertDir           string `json:"EtcdCertDir"`
}

type ClientInterface interface {
	GetStatus(ctx context.Context) (*K0sStatus, error)
}

func GetStatus(ctx context.Context) (*K0sStatus, error) {
	return _k0s.GetStatus(ctx)
}
