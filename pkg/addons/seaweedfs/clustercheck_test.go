package seaweedfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseClusterCheckOutput(t *testing.T) {
	// output header and non filer to filer checks as printed by weed shell 4.48
	header := `Topology volumeSizeLimit:1024 MB hdd(volume:6/24 active:6 free:18 remote:0)
the cluster has 3 filers: [10.0.0.1:8888 10.0.0.2:8888 10.0.0.3:8888]
the cluster has 3 volume servers: [10.0.0.4:8080 10.0.0.5:8080 10.0.0.6:8080]
checking master seaweedfs-master-0.seaweedfs-master.seaweedfs:9333 to volume server 10.0.0.4:8080.18080 ... ok round trip 1.062ms clock delta 0.407ms
checking filer 10.0.0.1:8888.18888 to master seaweedfs-master-0.seaweedfs-master.seaweedfs:9333 ... ok round trip 0.116ms clock delta 0.029ms
checking filer 10.0.0.1:8888.18888 to volume server 10.0.0.4:8080.18080 ... rpc error: code = InvalidArgument desc = unknown ping target 10.0.0.4:8080.18080 of type volumeServer
`

	tests := []struct {
		name         string
		output       string
		wantErr      bool
		errSubstr    []string
		notErrSubstr []string
	}{
		{
			name: "all filer to filer checks ok",
			output: header + `checking filer 10.0.0.1:8888.18888 to 10.0.0.1:8888.18888 ... ok round trip 0.193ms clock delta 0.026ms
checking filer 10.0.0.1:8888.18888 to 10.0.0.2:8888.18888 ... ok round trip 0.293ms clock delta 0.126ms
checking filer 10.0.0.2:8888.18888 to 10.0.0.1:8888.18888 ... ok round trip 0.393ms clock delta 0.226ms
checking filer 10.0.0.2:8888.18888 to 10.0.0.2:8888.18888 ... ok round trip 0.493ms clock delta 0.326ms
`,
			wantErr: false,
		},
		{
			name: "filer without peers",
			output: header + `checking filer 10.0.0.1:8888.18888 to 10.0.0.1:8888.18888 ... ok round trip 0.193ms clock delta 0.026ms
checking filer 10.0.0.1:8888.18888 to 10.0.0.2:8888.18888 ... rpc error: code = InvalidArgument desc = unknown ping target 10.0.0.2:8888.18888 of type filer
checking filer 10.0.0.2:8888.18888 to 10.0.0.1:8888.18888 ... ok round trip 0.393ms clock delta 0.226ms
checking filer 10.0.0.2:8888.18888 to 10.0.0.2:8888.18888 ... ok round trip 0.493ms clock delta 0.326ms
`,
			wantErr:   true,
			errSubstr: []string{"filer to filer checks failed", "unknown ping target 10.0.0.2:8888.18888 of type filer"},
			// failures in checks other than filer to filer are ignored
			notErrSubstr: []string{"of type volumeServer"},
		},
		{
			name: "filer unreachable",
			output: header + `checking filer 10.0.0.1:8888.18888 to 10.0.0.2:8888.18888 ... rpc error: code = Unavailable desc = connection error
`,
			wantErr:   true,
			errSubstr: []string{"code = Unavailable"},
		},
		{
			name:      "no filer to filer checks",
			output:    header,
			wantErr:   true,
			errSubstr: []string{"no filer to filer checks"},
		},
		{
			name:      "empty output",
			output:    "",
			wantErr:   true,
			errSubstr: []string{"no filer to filer checks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseClusterCheckOutput(tt.output)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, s := range tt.errSubstr {
				assert.Contains(t, err.Error(), s)
			}
			for _, s := range tt.notErrSubstr {
				assert.NotContains(t, err.Error(), s)
			}
		})
	}
}
