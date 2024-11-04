package helm

import (
	"reflect"
	"testing"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetValue(tt.args.values, tt.args.path, tt.args.newValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetValue() = %v, want %v", got, tt.want)
			}
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
