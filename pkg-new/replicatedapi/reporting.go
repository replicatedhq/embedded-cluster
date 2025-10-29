package replicatedapi

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

const (
	DistributionEmbeddedCluster string = "embedded-cluster"
)

type ReportingInfo struct {
	EmbeddedClusterNodes      *string `header:"X-Replicated-EmbeddedClusterNodes"`
	ReplHelmInstalls          *string `header:"X-Replicated-ReplHelmInstalls"`
	NativeHelmInstalls        *string `header:"X-Replicated-NativeHelmInstalls"`
	AppStatus                 *string `header:"X-Replicated-AppStatus"`
	InstallStatus             *string `header:"X-Replicated-InstallStatus"`
	PreflightStatus           *string `header:"X-Replicated-PreflightStatus"`
	DownstreamChannelSequence *string `header:"X-Replicated-DownstreamChannelSequence"`
	DownstreamSequence        *string `header:"X-Replicated-DownstreamSequence"`
	DownstreamSource          *string `header:"X-Replicated-DownstreamSource"`
	SkipPreflights            *string `header:"X-Replicated-SkipPreflights"`

	// unsupported headers
	// X-Replicated-KotsInstallID
	// X-Replicated-KurlInstallID
	// X-Replicated-KurlNodeCountTotal
	// X-Replicated-KurlNodeCountReady
	// X-Replicated-IsGitOpsEnabled
	// X-Replicated-GitOpsProvider
	// X-Replicated-SnapshotProvider
	// X-Replicated-SnapshotFullSchedule
	// X-Replicated-SnapshotFullTTL
	// X-Replicated-SnapshotPartialSchedule
	// X-Replicated-SnapshotPartialTTL

}

func (c *client) injectReportingInfoHeaders(header http.Header, reportingInfo *ReportingInfo) {
	for key, value := range c.getReportingInfoHeaders(reportingInfo) {
		header.Set(key, value)
	}
}

func (c *client) getReportingInfoHeaders(reportingInfo *ReportingInfo) map[string]string {
	headers := make(map[string]string)

	// add headers from client
	channel, _ := c.getChannelFromLicense() // ignore error
	if channel != nil {
		headers["X-Replicated-DownstreamChannelID"] = channel.ChannelID
		headers["X-Replicated-DownstreamChannelName"] = channel.ChannelName
	}

	headers["X-Replicated-K8sVersion"] = versions.K0sVersion
	headers["X-Replicated-K8sDistribution"] = DistributionEmbeddedCluster
	headers["X-Replicated-EmbeddedClusterVersion"] = versions.Version

	// TODO
	// headers["X-Replicated-ClusterID"] = "TODO"
	// headers["X-Replicated-InstanceID"] = "TODO"
	headers["X-Replicated-EmbeddedClusterID"] = c.clusterID

	// Add static headers
	headers["X-Replicated-IsKurl"] = "false"

	if reportingInfo != nil {
		// Use reflection to read struct tags and map fields to headers
		v := reflect.ValueOf(reportingInfo).Elem()
		t := v.Type()

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			headerName := field.Tag.Get("header")
			if headerName == "" {
				continue
			}

			fieldValue := v.Field(i)

			// Check if the pointer field is nil (not set)
			if fieldValue.IsNil() {
				continue
			}

			// Dereference the pointer to get the actual value
			actualValue := fieldValue.Elem()
			var strValue string

			switch actualValue.Kind() {
			case reflect.String:
				strValue = actualValue.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				strValue = fmt.Sprintf("%d", actualValue.Int())
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				strValue = fmt.Sprintf("%d", actualValue.Uint())
			case reflect.Bool:
				strValue = fmt.Sprintf("%t", actualValue.Bool())
			default:
				panic(fmt.Sprintf("reporting info field %s has unsupported type: %s", field.Name, actualValue.Kind()))
			}
			headers[headerName] = strValue
		}
	}

	// remove empty headers
	for key, value := range headers {
		if value == "" {
			delete(headers, key)
		}
	}

	return headers
}
