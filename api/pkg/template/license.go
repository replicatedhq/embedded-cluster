package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func (e *Engine) licenseFieldValue(name string) string {
	if e.license == nil {
		return ""
	}

	// Update docs at https://github.com/replicatedhq/kots.io/blob/main/content/reference/template-functions/license-context.md
	// when adding new values
	switch name {
	case "isSnapshotSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSnapshotSupported)
	case "IsDisasterRecoverySupported":
		return fmt.Sprintf("%t", e.license.Spec.IsDisasterRecoverySupported)
	case "isGitOpsSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGitOpsSupported)
	case "isSupportBundleUploadSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSupportBundleUploadSupported)
	case "isEmbeddedClusterMultiNodeEnabled":
		return fmt.Sprintf("%t", e.license.Spec.IsEmbeddedClusterMultiNodeEnabled)
	case "isIdentityServiceSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsIdentityServiceSupported)
	case "isGeoaxisSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGeoaxisSupported)
	case "isAirgapSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsAirgapSupported)
	case "licenseType":
		return e.license.Spec.LicenseType
	case "licenseSequence":
		return fmt.Sprintf("%d", e.license.Spec.LicenseSequence)
	case "signature":
		return string(e.license.Spec.Signature)
	case "appSlug":
		return e.license.Spec.AppSlug
	case "channelID":
		return e.license.Spec.ChannelID
	case "channelName":
		return e.license.Spec.ChannelName
	case "isSemverRequired":
		return fmt.Sprintf("%t", e.license.Spec.IsSemverRequired)
	case "customerName":
		return e.license.Spec.CustomerName
	case "licenseID", "licenseId":
		return e.license.Spec.LicenseID
	case "endpoint":
		if e.releaseData == nil {
			return ""
		}
		ecDomains := utils.GetDomains(e.releaseData)
		return netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain)
	default:
		entitlement, ok := e.license.Spec.Entitlements[name]
		if ok {
			return fmt.Sprintf("%v", entitlement.Value.Value())
		}
		return ""
	}
}

func (e *Engine) licenseDockerCfg() (string, error) {
	if e.license == nil {
		return "", fmt.Errorf("license is nil")
	}
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}

	auth := fmt.Sprintf("%s:%s", e.license.Spec.LicenseID, e.license.Spec.LicenseID)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))

	registryProxyInfo := getRegistryProxyInfo(e.releaseData)

	dockercfg := map[string]any{
		"auths": map[string]any{
			registryProxyInfo.Proxy: map[string]string{
				"auth": encodedAuth,
			},
			registryProxyInfo.Registry: map[string]string{
				"auth": encodedAuth,
			},
		},
	}

	b, err := json.Marshal(dockercfg)
	if err != nil {
		return "", fmt.Errorf("marshal dockercfg: %w", err)
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

type registryProxyInfo struct {
	Registry string
	Proxy    string
}

func getRegistryProxyInfo(releaseData *release.ReleaseData) *registryProxyInfo {
	ecDomains := utils.GetDomains(releaseData)
	return &registryProxyInfo{
		Proxy:    ecDomains.ProxyRegistryDomain,
		Registry: ecDomains.ReplicatedRegistryDomain,
	}
}
