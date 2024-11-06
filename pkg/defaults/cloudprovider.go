package defaults

import (
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

// TryDiscoverPublicIP tries to discover the public IP of the node by querying
// a list of known providers. If the public IP cannot be discovered, an empty
// string is returned.
func TryDiscoverPublicIP() string {
	if !shouldUseMetadataService() {
		return ""
	}

	publicIP := tryDiscoverPublicIPAWSIMDSv2()
	if publicIP != "" {
		return publicIP
	}

	publicIP = tryDiscoverPublicIPAWSIMDSv1()
	if publicIP != "" {
		return publicIP
	}

	publicIP = tryDiscoverPublicIPGCE()
	if publicIP != "" {
		return publicIP
	}

	publicIP = tryDiscoverPublicIPAzure()
	if publicIP != "" {
		return publicIP
	}

	// If we reach this point, we failed to discover the public IP.
	return ""
}

func tryDiscoverPublicIPGCE() string {
	return makeMetadataRequest(
		http.MethodGet,
		"http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip",
		map[string]string{"Metadata-Flavor": "Google"},
	)
}

func tryDiscoverPublicIPAWSIMDSv1() string {
	return makeMetadataRequest(
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
	return makeMetadataRequest(
		http.MethodGet,
		"http://169.254.169.254/latest/meta-data/public-ipv4",
		map[string]string{"X-aws-ec2-metadata-token": token},
	)
}

func tryDiscoverPublicIPAzure() string {
	return makeMetadataRequest(
		http.MethodGet,
		"http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text",
		map[string]string{"Metadata": "true"},
	)
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
	publicIP := string(bodyBytes)
	if net.ParseIP(publicIP).To4() != nil {
		return publicIP
	}
	return ""
}