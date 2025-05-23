package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestAppendFieldError(t *testing.T) {
	tests := []struct {
		name string
		fe   func() *APIError
		want string
	}{
		{
			name: "empty",
			fe: func() *APIError {
				return nil
			},
			want: "",
		},
		{
			name: "single error",
			fe: func() *APIError {
				var fe *APIError
				fe = AppendFieldError(fe, "field1", errors.New("error1"))
				b, _ := json.Marshal(fe)
				fmt.Println(string(b))
				return fe
			},
			want: "field errors: field1: error1",
		},
		{
			name: "multiple errors",
			fe: func() *APIError {
				var fe *APIError
				fe = AppendFieldError(fe, "field1", errors.New("error1"))
				fe = AppendFieldError(fe, "field2", errors.New("error2"))
				return fe
			},
			want: "field errors: field1: error1; field2: error2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := tt.fe()
			if got := fe.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
