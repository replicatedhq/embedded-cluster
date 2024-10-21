package main

import (
	"context"
	"encoding/json"
	"os/exec"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

var (
	k0s = "/usr/local/bin/k0s"
)

type k0sStatus struct {
	Role          string                   `json:"Role"`
	Vars          k0sVars                  `json:"K0sVars"`
	ClusterConfig k0sv1beta1.ClusterConfig `json:"ClusterConfig"`
}

type k0sVars struct {
	AdminKubeConfigPath   string `json:"AdminKubeConfigPath"`
	KubeletAuthConfigPath string `json:"KubeletAuthConfigPath"`
	CertRootDir           string `json:"CertRootDir"`
	EtcdCertDir           string `json:"EtcdCertDir"`
}

// getK0sStatus returns the status of the k0s service.
func getK0sStatus(ctx context.Context) (*k0sStatus, error) {
	// get k0s status json
	out, err := exec.CommandContext(ctx, k0s, "status", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var status k0sStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
