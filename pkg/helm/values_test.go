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
