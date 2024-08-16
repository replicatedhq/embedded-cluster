package release

import (
	"bytes"
	"fmt"
	"text/template"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type TemplateData struct {
	IsAirgap              bool
	ReplicatedProxyDomain string
}

func Template(raw string, license *kotsv1beta1.License, isAirgap bool) (string, error) {
	tmpl, err := template.New("release").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	data, err := getTemplateData(license, isAirgap)
	if err != nil {
		return "", fmt.Errorf("get template data: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func getTemplateData(license *kotsv1beta1.License, isAirgap bool) (*TemplateData, error) {
	data := defaultTemplateData(isAirgap)
	if license == nil {
		return data, nil
	}
	rel, err := GetChannelRelease()
	if err != nil {
		return nil, fmt.Errorf("unable to get channel release: %w", err)
	}
	if rel == nil {
		return data, nil
	}
	ch := findChannelInLicense(rel.ChannelSlug, license)
	if ch == nil {
		return data, nil
	}
	if ch.ReplicatedProxyDomain != "" {
		data.ReplicatedProxyDomain = ch.ReplicatedProxyDomain
	}
	return data, nil
}

func defaultTemplateData(isAirgap bool) *TemplateData {
	return &TemplateData{
		IsAirgap:              isAirgap,
		ReplicatedProxyDomain: "proxy.replicated.com/anonymous/",
	}
}

// TODO NOW: write tests for this
func findChannelInLicense(channelSlug string, license *kotsv1beta1.License) *kotsv1beta1.Channel {
	for _, ch := range license.Spec.Channels {
		if ch.ChannelSlug == channelSlug {
			return &ch
		}
	}
	return nil
}
