package kotscli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getLastDeployedAppVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []AppVersionInfo
		want     *AppVersionInfo
		wantErr  bool
	}{
		{
			name: "should return first deployed version",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "1.0.0",
					ChannelSequence: 1,
					Sequence:        1,
					Status:          "deployed",
				},
				{
					VersionLabel:    "0.9.0",
					ChannelSequence: 2,
					Sequence:        2,
					Status:          "pending",
				},
			},
			want: &AppVersionInfo{
				VersionLabel:    "1.0.0",
				ChannelSequence: 1,
				Sequence:        1,
				Status:          "deployed",
			},
			wantErr: false,
		},
		{
			name: "should return failed deployment",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "1.0.0",
					ChannelSequence: 1,
					Sequence:        1,
					Status:          "failed",
				},
				{
					VersionLabel:    "0.9.0",
					ChannelSequence: 2,
					Sequence:        2,
					Status:          "pending",
				},
			},
			want: &AppVersionInfo{
				VersionLabel:    "1.0.0",
				ChannelSequence: 1,
				Sequence:        1,
				Status:          "failed",
			},
			wantErr: false,
		},
		{
			name: "should return deployed before failed when both exist",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "2.0.0",
					ChannelSequence: 3,
					Sequence:        3,
					Status:          "deployed",
				},
				{
					VersionLabel:    "1.5.0",
					ChannelSequence: 2,
					Sequence:        2,
					Status:          "failed",
				},
			},
			want: &AppVersionInfo{
				VersionLabel:    "2.0.0",
				ChannelSequence: 3,
				Sequence:        3,
				Status:          "deployed",
			},
			wantErr: false,
		},
		{
			name: "should return failed when it comes before deployed",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "2.0.0",
					ChannelSequence: 3,
					Sequence:        3,
					Status:          "failed",
				},
				{
					VersionLabel:    "1.5.0",
					ChannelSequence: 2,
					Sequence:        2,
					Status:          "deployed",
				},
			},
			want: &AppVersionInfo{
				VersionLabel:    "2.0.0",
				ChannelSequence: 3,
				Sequence:        3,
				Status:          "failed",
			},
			wantErr: false,
		},
		{
			name: "should return error when no deployed or failed versions exist",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "1.0.0",
					ChannelSequence: 1,
					Sequence:        1,
					Status:          "pending",
				},
				{
					VersionLabel:    "0.9.0",
					ChannelSequence: 2,
					Sequence:        2,
					Status:          "pending_download",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:     "should return error when versions slice is empty",
			versions: []AppVersionInfo{},
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "should return error when versions slice is nil",
			versions: nil,
			want:     nil,
			wantErr:  true,
		},
		{
			name: "should handle versions with other statuses",
			versions: []AppVersionInfo{
				{
					VersionLabel:    "3.0.0",
					ChannelSequence: 5,
					Sequence:        5,
					Status:          "pending_config",
				},
				{
					VersionLabel:    "2.5.0",
					ChannelSequence: 4,
					Sequence:        4,
					Status:          "pending_download",
				},
				{
					VersionLabel:    "2.0.0",
					ChannelSequence: 3,
					Sequence:        3,
					Status:          "deployed",
				},
				{
					VersionLabel:    "1.0.0",
					ChannelSequence: 1,
					Sequence:        1,
					Status:          "unknown",
				},
			},
			want: &AppVersionInfo{
				VersionLabel:    "2.0.0",
				ChannelSequence: 3,
				Sequence:        3,
				Status:          "deployed",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getLastDeployedAppVersion(tt.versions)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
