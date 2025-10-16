package template

import "fmt"

func (e *Engine) versionLabel() (string, error) {
	if e.releaseData == nil {
		return "", fmt.Errorf("release data is nil")
	}
	if e.releaseData.ChannelRelease == nil {
		return "", fmt.Errorf("channel release is nil")
	}
	return e.releaseData.ChannelRelease.VersionLabel, nil
}
