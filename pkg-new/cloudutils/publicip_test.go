package cloudutils

import "testing"

func Test_parseAzureLoadBalancerMetadataResponse(t *testing.T) {
	type args struct {
		resp string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				resp: `{"loadbalancer":{"publicIpAddresses":[{"frontendIpAddress":"52.249.223.56","privateIpAddress":"10.0.0.4"}],"inboundRules":[],"outboundRules":[]}}`,
			},
			want:    "52.249.223.56",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAzureLoadBalancerMetadataResponse(tt.args.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAzureLoadBalancerMetadataResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseAzureLoadBalancerMetadataResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
