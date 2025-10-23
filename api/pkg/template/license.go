package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func (e *Engine) licenseFieldValue(name string) (string, error) {
	if e.license == nil {
		return "", fmt.Errorf("license is nil")
	}

	// Update docs at https://github.com/replicatedhq/kots.io/blob/main/content/reference/template-functions/license-context.md
	// when adding new values
	switch name {
	case "isSnapshotSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSnapshotSupported), nil
	case "IsDisasterRecoverySupported":
		return fmt.Sprintf("%t", e.license.Spec.IsDisasterRecoverySupported), nil
	case "isGitOpsSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGitOpsSupported), nil
	case "isSupportBundleUploadSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSupportBundleUploadSupported), nil
	case "isEmbeddedClusterMultiNodeEnabled":
		return fmt.Sprintf("%t", e.license.Spec.IsEmbeddedClusterMultiNodeEnabled), nil
	case "isIdentityServiceSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsIdentityServiceSupported), nil
	case "isGeoaxisSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGeoaxisSupported), nil
	case "isAirgapSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsAirgapSupported), nil
	case "licenseType":
		return e.license.Spec.LicenseType, nil
	case "licenseSequence":
		return fmt.Sprintf("%d", e.license.Spec.LicenseSequence), nil
	case "signature":
		return string(e.license.Spec.Signature), nil
	case "appSlug":
		return e.license.Spec.AppSlug, nil
	case "channelID":
		return e.license.Spec.ChannelID, nil
	case "channelName":
		return e.license.Spec.ChannelName, nil
	case "isSemverRequired":
		return fmt.Sprintf("%t", e.license.Spec.IsSemverRequired), nil
	case "customerName":
		return e.license.Spec.CustomerName, nil
	case "licenseID", "licenseId":
		return e.license.Spec.LicenseID, nil
	case "endpoint":
		if e.releaseData == nil {
			return "", fmt.Errorf("release data is nil")
		}
		ecDomains := utils.GetDomains(e.releaseData)
		return netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain), nil
	default:
		entitlement, ok := e.license.Spec.Entitlements[name]
		if ok {
			return fmt.Sprintf("%v", entitlement.Value.Value()), nil
		}
		return "", nil
	}
}

func (e *Engine) licenseDockerCfg() (string, error) {
	if e.license == nil {
		return "", fmt.Errorf("license is nil")
	}
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}
	if e.releaseData.ChannelRelease == nil {
		return "", fmt.Errorf("channel release is nil")
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

func (e *Engine) channelName() (string, error) {
	if e.license == nil {
		return "", fmt.Errorf("license is nil")
	}
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}
	if e.releaseData.ChannelRelease == nil {
		return "", fmt.Errorf("channel release is nil")
	}

	for _, channel := range e.license.Spec.Channels {
		if channel.ChannelID == e.releaseData.ChannelRelease.ChannelID {
			return channel.ChannelName, nil
		}
	}
	if e.license.Spec.ChannelID == e.releaseData.ChannelRelease.ChannelID {
		return e.license.Spec.ChannelName, nil
	}
	return "", fmt.Errorf("channel %s not found in license", e.releaseData.ChannelRelease.ChannelID)
}
