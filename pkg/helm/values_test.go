package helm

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetValue(t *testing.T) {
	type args struct {
		values   map[string]interface{}
		path     string
		newValue interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "set value",
			args: args{
				values: map[string]interface{}{
					"foo": "bar",
				},
				path:     "foo",
				newValue: "new value",
			},
			want: map[string]interface{}{
				"foo": "new value",
			},
			wantErr: false,
		},
		{
			name: "set value in nested map",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				},
				path:     "foo.bar",
				newValue: "new value",
			},
			want: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "new value",
				},
			},
			wantErr: false,
		},
		{
			name: "set value in empty map",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{},
				},
				path:     "foo.bar",
				newValue: "new value",
			},
			want: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "new value",
				},
			},
			wantErr: false,
		},
		{
			name: "test create new map",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "new value",
					},
				},
				path:     "foo.baz",
				newValue: map[string]interface{}{"buzz": "fizz"},
			},
			want: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "new value",
					"baz": map[string]interface{}{
						"buzz": "fizz",
					},
				},
			},
		},
		{
			name: `can set adminconsole "kurlProxy.nodePort"`,
			args: args{
				values: map[string]interface{}{
					"kurlProxy": map[string]interface{}{
						"enabled":  true,
						"nodePort": 30000,
					},
				},
				path:     "kurlProxy.nodePort",
				newValue: 30001,
			},
			want: map[string]interface{}{
				"kurlProxy": map[string]interface{}{
					"enabled":  true,
					"nodePort": float64(30001),
				},
			},
			wantErr: false,
		},
		{
			name: `can set openebs "localpv-provisioner.localpv.basePath"`,
			args: args{
				values: map[string]interface{}{
					"localpv-provisioner": map[string]interface{}{
						"analytics": map[string]interface{}{
							"enabled": false,
						},
						"localpv": map[string]interface{}{
							"image": map[string]interface{}{
								"registry": "proxy.replicated.com/anonymous/",
							},
							"basePath": "/var/lib/embedded-cluster/openebs-local",
						},
					},
				},
				path:     "localpv-provisioner.localpv.basePath",
				newValue: "/var/ec/openebs-local",
			},
			want: map[string]interface{}{
				"localpv-provisioner": map[string]interface{}{
					"analytics": map[string]interface{}{
						"enabled": false,
					},
					"localpv": map[string]interface{}{
						"image": map[string]interface{}{
							"registry": "proxy.replicated.com/anonymous/",
						},
						"basePath": "/var/ec/openebs-local",
					},
				},
			},
			wantErr: false,
		},
		{
			name: `can set seaweedfs "master.data.hostPathPrefix"`,
			args: args{
				values: map[string]interface{}{
					"master": map[string]interface{}{
						"replicas":     1,
						"nodeSelector": nil,
						"data": map[string]interface{}{
							"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/ssd",
						},
						"logs": map[string]interface{}{
							"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/storage",
						},
					},
				},
				path:     "master.data.hostPathPrefix",
				newValue: "/var/ec/seaweedfs/ssd",
			},
			want: map[string]interface{}{
				"master": map[string]interface{}{
					"replicas":     1,
					"nodeSelector": nil,
					"data": map[string]interface{}{
						"hostPathPrefix": "/var/ec/seaweedfs/ssd",
					},
					"logs": map[string]interface{}{
						"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/storage",
					},
				},
			},
			wantErr: false,
		},
		{
			name: `can set seaweedfs "master.logs.hostPathPrefix"`,
			args: args{
				values: map[string]interface{}{
					"master": map[string]interface{}{
						"replicas":     1,
						"nodeSelector": nil,
						"data": map[string]interface{}{
							"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/ssd",
						},
						"logs": map[string]interface{}{
							"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/storage",
						},
					},
				},
				path:     "master.logs.hostPathPrefix",
				newValue: "/var/ec/seaweedfs/storage",
			},
			want: map[string]interface{}{
				"master": map[string]interface{}{
					"replicas":     1,
					"nodeSelector": nil,
					"data": map[string]interface{}{
						"hostPathPrefix": "/var/lib/embedded-cluster/seaweedfs/ssd",
					},
					"logs": map[string]interface{}{
						"hostPathPrefix": "/var/ec/seaweedfs/storage",
					},
				},
			},
			wantErr: false,
		},
		{
			name: `can set velero "nodeAgent.podVolumePath"`,
			args: args{
				values: map[string]interface{}{
					"nodeAgent": map[string]interface{}{
						"podVolumePath": "/var/lib/embedded-cluster/k0s/kubelet/pods",
					},
					"snapshotsEnabled": false,
				},
				path:     "nodeAgent.podVolumePath",
				newValue: "/var/ec/k0s/kubelet/pods",
			},
			want: map[string]interface{}{
				"nodeAgent": map[string]interface{}{
					"podVolumePath": "/var/ec/k0s/kubelet/pods",
				},
				"snapshotsEnabled": false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetValue(tt.args.values, tt.args.path, tt.args.newValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, tt.args.values)
		})
	}
}

func TestGetValue(t *testing.T) {
	type args struct {
		values map[string]interface{}
		path   string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "get value",
			args: args{
				values: map[string]interface{}{
					"foo": "bar",
				},
				path: "foo",
			},
			want: "bar",
		},
		{
			name: "get value from array",
			args: args{
				values: map[string]interface{}{
					"foo": []interface{}{"bar", "baz"},
				},
				path: "foo[0]",
			},
			want: "bar",
		},
		{
			name: "get value from nested map",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				},
				path: "foo.bar",
			},
			want: "baz",
		},
		{
			name: "get value from nested array",
			args: args{
				values: map[string]interface{}{
					"foo": []interface{}{
						map[string]interface{}{
							"bar": []interface{}{
								"baz",
							},
						},
					},
				},
				path: "foo[0].bar[0]",
			},
			want: "baz",
		},
		{
			name: "get value from missing map",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				},
				path: "foo.bar.baz",
			},
			wantErr: true,
		},
		{
			name: "get value for key with hyphen",
			args: args{
				values: map[string]interface{}{
					"foo-bar": "baz",
				},
				path: "['foo-bar']",
			},
			want: "baz",
		},
		{
			name: "get value for nested key with hyphen",
			args: args{
				values: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar-baz": "baz",
					},
				},
				path: "foo['bar-baz']",
			},
			want: "baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(tt.args.values, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
