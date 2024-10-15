package types

import (
	"encoding/json"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DryRun struct {
	Flags    map[string]interface{} `json:"flags"`
	Commands []Command              `json:"commands"`
	Metrics  []interface{}          `json:"metrics"`
	Data     map[string]interface{} `json:"data"`

	// TODO NOW: logs
	KCLI client.Client `json:"-"`
}

type Command struct {
	Cmd  string            `json:"cmd"`
	Args []string          `json:"args,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

func (d *DryRun) MarshalJSON() ([]byte, error) {
	type Alias DryRun

	return json.Marshal(&struct {
		*Alias
		OSEnv []string `json:"osEnv"`
		// K8sObjects string   `json:"k8sObjects"`
	}{
		Alias: (*Alias)(d),
		OSEnv: os.Environ(),
		// K8sObjects: d.K8sObjects,
	})
}
