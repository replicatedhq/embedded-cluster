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
	if e.license.IsEmpty() {
		return "", fmt.Errorf("license is nil")
	}

	// Update docs at https://github.com/replicatedhq/kots.io/blob/main/content/reference/template-functions/license-context.md
	// when adding new values
	switch name {
	case "isSnapshotSupported":
		return fmt.Sprintf("%t", e.license.IsSnapshotSupported()), nil
	case "IsDisasterRecoverySupported":
		return fmt.Sprintf("%t", e.license.IsDisasterRecoverySupported()), nil
	case "isGitOpsSupported":
		return fmt.Sprintf("%t", e.license.IsGitOpsSupported()), nil
	case "isSupportBundleUploadSupported":
		return fmt.Sprintf("%t", e.license.IsSupportBundleUploadSupported()), nil
	case "isEmbeddedClusterMultiNodeEnabled":
		return fmt.Sprintf("%t", e.license.IsEmbeddedClusterMultiNodeEnabled()), nil
	case "isIdentityServiceSupported":
		return fmt.Sprintf("%t", e.license.IsIdentityServiceSupported()), nil
	case "isGeoaxisSupported":
		return fmt.Sprintf("%t", e.license.IsGeoaxisSupported()), nil
	case "isAirgapSupported":
		return fmt.Sprintf("%t", e.license.IsAirgapSupported()), nil
	case "licenseType":
		return e.license.GetLicenseType(), nil
	case "licenseSequence":
		return fmt.Sprintf("%d", e.license.GetLicenseSequence()), nil
	case "signature":
		return string(e.license.GetSignature()), nil
	case "appSlug":
		return e.license.GetAppSlug(), nil
	case "channelID":
		return e.license.GetChannelID(), nil
	case "channelName":
		return e.license.GetChannelName(), nil
	case "isSemverRequired":
		return fmt.Sprintf("%t", e.license.IsSemverRequired()), nil
	case "customerName":
		return e.license.GetCustomerName(), nil
	case "licenseID", "licenseId":
		return e.license.GetLicenseID(), nil
	case "endpoint":
		if e.releaseData == nil {
			return "", fmt.Errorf("release data is nil")
		}
		ecDomains := utils.GetDomains(e.releaseData)
		return netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain), nil
	default:
		entitlements := e.license.GetEntitlements()
		entitlement, ok := entitlements[name]
		if ok {
			val := entitlement.GetValue()
			return fmt.Sprintf("%v", val.Value()), nil
		}
		return "", nil
	}
}

func (e *Engine) licenseDockerCfg() (string, error) {
	if e.license.IsEmpty() {
		return "", fmt.Errorf("license is nil")
	}
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}
	if e.releaseData.ChannelRelease == nil {
		return "", fmt.Errorf("channel release is nil")
	}

	licenseID := e.license.GetLicenseID()
	auth := fmt.Sprintf("%s:%s", licenseID, licenseID)
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
	if e.license.IsEmpty() {
		return "", fmt.Errorf("license is nil")
	}
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}
	if e.releaseData.ChannelRelease == nil {
		return "", fmt.Errorf("channel release is nil")
	}

	for _, channel := range e.license.GetChannels() {
		if channel.ChannelID == e.releaseData.ChannelRelease.ChannelID {
			return channel.ChannelName, nil
		}
	}
	if e.license.GetChannelID() == e.releaseData.ChannelRelease.ChannelID {
		return e.license.GetChannelName(), nil
	}
	return "", fmt.Errorf("channel %s not found in license", e.releaseData.ChannelRelease.ChannelID)
}
