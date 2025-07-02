package seaweedfs

import (
	"context"
	_ "embed"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
)

func (s *SeaweedFS) GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error) {
	hv, err := helmValues()
	if err != nil {
		return nil, errors.Wrap(err, "get helm values")
	}

	marshalled, err := helm.MarshalValues(hv)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if domains.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", domains.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	dataPath := filepath.Join(s.SeaweedFSDataDir, "ssd")
	err = helm.SetValue(copiedValues, "master.data.hostPathPrefix", dataPath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm values global.data.hostPathPrefix")
	}

	logsPath := filepath.Join(s.SeaweedFSDataDir, "storage")
	err = helm.SetValue(copiedValues, "master.logs.hostPathPrefix", logsPath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm values global.logs.hostPathPrefix")
	}

	for _, override := range overrides {
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}

func helmValues() (map[string]interface{}, error) {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata")
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	return hv, nil
}
