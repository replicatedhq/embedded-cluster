package migratev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_updateAdminConsoleChartValues(t *testing.T) {
	type args struct {
		values []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				values: []byte(`isAirgap: "true"
embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f830064e
`),
			},
			want: []byte(`embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f830064e
isAirgap: "true"
isEC2Install: "true"
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updateAdminConsoleChartValues(tt.args.values)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, string(tt.want), string(got))
		})
	}
}
