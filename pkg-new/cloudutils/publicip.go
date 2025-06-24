package cloudutils

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	noProxyTransport *http.Transport
)

func init() {
	noProxyTransport = http.DefaultTransport.(*http.Transport).Clone()
	noProxyTransport.Proxy = nil // no proxy
}

// TryDiscoverPublicIP tries to discover the public IP of the node by querying a list of known
// providers. If the public IP cannot be discovered, an empty string is returned.
func (c *CloudUtils) TryDiscoverPublicIP() string {
	if !shouldUseMetadataService() {
		c.logger.Debug("No cloud provider metadata service found, skipping public IP discovery")
		return ""
	}

	publicIP := tryDiscoverPublicIPAWSIMDSv2()
	if publicIP != "" {
		c.logger.Debugf("Found public IP %s using AWS IMDSv2", publicIP)
		return publicIP
	}

	publicIP = tryDiscoverPublicIPAWSIMDSv1()
	if publicIP != "" {
		c.logger.Debugf("Found public IP %s using AWS IMDSv1", publicIP)
		return publicIP
	}

	publicIP = tryDiscoverPublicIPGCE()
	if publicIP != "" {
		c.logger.Debugf("Found public IP %s using GCE", publicIP)
		return publicIP
	}

	publicIP = tryDiscoverPublicIPAzureStandardSKU()
	if publicIP != "" {
		c.logger.Debugf("Found public IP %s using Azure Standard SKU", publicIP)
		return publicIP
	}

	publicIP = tryDiscoverPublicIPAzure()
	if publicIP != "" {
		c.logger.Debugf("Found public IP %s using Azure", publicIP)
		return publicIP
	}

	return ""
}

func tryDiscoverPublicIPGCE() string {
	return makeMetadataRequestForIPv4(
		http.MethodGet,
		"http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip",
		map[string]string{"Metadata-Flavor": "Google"},
	)
}

func tryDiscoverPublicIPAWSIMDSv1() string {
	return makeMetadataRequestForIPv4(
		http.MethodGet,
		"http://169.254.169.254/latest/meta-data/public-ipv4",
		nil,
	)
}

func tryDiscoverPublicIPAWSIMDSv2() string {
	token := makeMetadataRequest(
		http.MethodPut,
		"http://169.254.169.254/latest/api/token",
		map[string]string{"X-aws-ec2-metadata-token-ttl-seconds": "60"},
	)
	if token == "" {
		return ""
	}
	return makeMetadataRequestForIPv4(
		http.MethodGet,
		"http://169.254.169.254/latest/meta-data/public-ipv4",
		map[string]string{"X-aws-ec2-metadata-token": token},
	)
}

func tryDiscoverPublicIPAzure() string {
	return makeMetadataRequestForIPv4(
		http.MethodGet,
		"http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text",
		map[string]string{"Metadata": "true"},
	)
}

// https://learn.microsoft.com/en-us/azure/load-balancer/howto-load-balancer-imds?tabs=windows
func tryDiscoverPublicIPAzureStandardSKU() string {
	resp := makeMetadataRequest(
		http.MethodGet,
		"http://169.254.169.254/metadata/loadbalancer?api-version=2020-10-01&format=text",
		map[string]string{"Metadata": "true"},
	)
	publicIP, _ := parseAzureLoadBalancerMetadataResponse(resp)
	return publicIP
}

func parseAzureLoadBalancerMetadataResponse(resp string) (string, error) {
	type loadBalancer struct {
		LoadBalancer struct {
			PublicIpAddresses []struct {
				FrontendIpAddress string `json:"frontendIpAddress"`
				PrivateIpAddress  string `json:"privateIpAddress"`
			} `json:"publicIpAddresses"`
		} `json:"loadbalancer"`
	}
	var lb loadBalancer
	if err := json.Unmarshal([]byte(resp), &lb); err != nil {
		return "", err
	}
	if len(lb.LoadBalancer.PublicIpAddresses) == 0 {
		return "", nil
	}
	return lb.LoadBalancer.PublicIpAddresses[0].FrontendIpAddress, nil
}

// shouldUseMetadataService returns true if the metadata service is available and responds with any
// status code. This is needed to speed up a failure in an air gapped environment where the request
// may timeout.
func shouldUseMetadataService() bool {
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: noProxyTransport,
	}

	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254", nil)
	if err != nil {
		logrus.Errorf("Unable to create metadata request: %v", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return true
}

func makeMetadataRequestForIPv4(method string, url string, headers map[string]string) string {
	body := makeMetadataRequest(method, url, headers)
	if net.ParseIP(body).To4() != nil {
		return body
	}
	return ""
}

func makeMetadataRequest(method string, url string, headers map[string]string) string {
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: noProxyTransport,
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		logrus.Errorf("Unable to create metadata request: %v", err)
		return ""
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return string(bodyBytes)
}
