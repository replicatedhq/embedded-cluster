package cmx

import (
	"fmt"
	"os"
)

func replicatedApiTokenEnv() ([]string, error) {
	if val := os.Getenv("CMX_REPLICATED_API_TOKEN"); val != "" {
		return []string{fmt.Sprintf("REPLICATED_API_TOKEN=%s", val), "REPLICATED_API_ORIGIN=", "REPLICATED_APP="}, nil
	}
	return nil, fmt.Errorf("CMX_REPLICATED_API_TOKEN is not set")
}
