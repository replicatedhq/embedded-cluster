package support

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type TemplateData struct {
	DataDir          string
	K0sDataDir       string
	OpenEBSDataDir   string
	LogsDir          string
	IsAirgap         bool
	ReplicatedAppURL string
	ProxyRegistryURL string
	HTTPProxy        string
	HTTPSProxy       string
	NoProxy          string
}

func MaterializeSupportBundleSpec(rc runtimeconfig.RuntimeConfig, isAirgap bool) error {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}
	domains := domains.GetDomains(embCfgSpec, nil)

	data := TemplateData{
		DataDir:          rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:       rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:   rc.EmbeddedClusterOpenEBSLocalSubDir(),
		LogsDir:          runtimeconfig.EmbeddedClusterLogsPath(),
		IsAirgap:         isAirgap,
		ReplicatedAppURL: netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain),
		ProxyRegistryURL: netutils.MaybeAddHTTPS(domains.ProxyRegistryDomain),
	}

	// Add proxy configuration if available
	if proxy := rc.ProxySpec(); proxy != nil {
		data.HTTPProxy = proxy.HTTPProxy
		data.HTTPSProxy = proxy.HTTPSProxy
		data.NoProxy = proxy.NoProxy
	}

	path := rc.PathToEmbeddedClusterSupportFile("host-support-bundle.tmpl.yaml")
	tmpl, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read support bundle template: %w", err)
	}
	contents, err := renderTemplate(string(tmpl), data)
	if err != nil {
		return fmt.Errorf("render support bundle template: %w", err)
	}
	path = rc.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		return fmt.Errorf("write support bundle spec: %w", err)
	}
	return nil
}

func renderTemplate(spec string, data TemplateData) (string, error) {
	tmpl, err := template.New("preflight").Parse(spec)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
