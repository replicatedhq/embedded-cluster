package release

import (
	"bytes"
	"fmt"
	"text/template"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type TemplateData struct {
	ReplicatedProxyDomain string
}

func Template(raw []byte, license *kotsv1beta1.License) ([]byte, error) {
	tmpl, err := template.New("release").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	data, err := getTemplateData(license)
	if err != nil {
		return nil, fmt.Errorf("get template data: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func getTemplateData(license *kotsv1beta1.License) (*TemplateData, error) {
	data := defaultTemplateData()
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

func defaultTemplateData() *TemplateData {
	return &TemplateData{
		ReplicatedProxyDomain: "proxy.replicated.com",
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
