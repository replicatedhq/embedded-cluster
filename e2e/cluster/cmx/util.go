package cmx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func replicatedApiTokenEnv() ([]string, error) {
	if val := os.Getenv("CMX_REPLICATED_API_TOKEN"); val != "" {
		return []string{fmt.Sprintf("REPLICATED_API_TOKEN=%s", val), "REPLICATED_API_ORIGIN=", "REPLICATED_APP="}, nil
	}
	return nil, fmt.Errorf("CMX_REPLICATED_API_TOKEN is not set")
}

func tmpFileName(pattern string) (string, error) {
	srcTar, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp file: %v", err)
	}
	name := srcTar.Name()
	err = srcTar.Close()
	if err != nil {
		return "", fmt.Errorf("close temp file: %v", err)
	}
	err = os.Remove(name)
	if err != nil {
		return "", fmt.Errorf("remove temp file: %v", err)
	}
	return name, nil
}

func tgzDir(ctx context.Context, src string, dst string) error {
	cmd := exec.CommandContext(ctx, "tar", "-czf", dst, filepath.Base(src))
	cmd.Dir = filepath.Dir(src)

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %v, stderr: %s", err, errBuf.String())
	}
	return nil
}
