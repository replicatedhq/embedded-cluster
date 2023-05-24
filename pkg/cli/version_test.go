package cli

import (
	"context"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestCmdVersionOptions_Run(t *testing.T) {
	tests := []struct {
		name    string
		wantOut string
		wantErr bool
	}{
		{
			name:    "basic",
			wantOut: "0.0.1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, buf, _ := genericclioptions.NewTestIOStreams()
			cli := &CLI{IOStreams: streams}
			o := &CmdVersionOptions{}
			if err := o.Run(context.Background(), cli); (err != nil) != tt.wantErr {
				t.Errorf("CmdVersionOptions.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotOut := buf.String(); gotOut != tt.wantOut {
				t.Errorf("CmdVersionOptions.Run() = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}
