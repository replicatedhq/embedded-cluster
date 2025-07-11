package helpers

import (
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestK0sClusterConfigTo129Compat(t *testing.T) {
	type args struct {
		clusterConfig *k0sv1beta1.ClusterConfig
	}

	tests := []struct {
		name           string
		args           args
		wantHelmCharts []interface{}
		wantErr        bool
	}{
		{
			name: "basic",
			args: args{
				clusterConfig: &k0sv1beta1.ClusterConfig{
					Spec: &k0sv1beta1.ClusterSpec{
						Extensions: &k0sv1beta1.ClusterExtensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:      "chart",
										ChartName: "chartname",
										Version:   "1.0.0",
										Values:    "values",
										TargetNS:  "targetns",
										Timeout:   time.Minute, // NOTE: this is not needed for 1.29 compat
										Order:     1,
									},
								},
							},
						},
					},
				},
			},
			wantHelmCharts: []interface{}{
				map[string]interface{}{
					"name":      "chart",
					"chartname": "chartname",
					"version":   "1.0.0",
					"values":    "values",
					"namespace": "targetns",
					"timeout":   time.Minute,
					"order":     float64(1),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := K0sClusterConfigTo129Compat(tt.args.clusterConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("clusterConfig129Compat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantHelmCharts, got.UnstructuredContent()["spec"].(map[string]interface{})["extensions"].(map[string]interface{})["helm"].(map[string]interface{})["charts"])
		})
	}
}
