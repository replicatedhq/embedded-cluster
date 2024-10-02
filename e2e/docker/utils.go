package docker

import (
	"math/rand"
	"os/exec"
	"testing"
)

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateID() string {
	b := make([]rune, 32)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return "ece2e-" + string(b)
}

func dockerBinPath(t *testing.T) string {
	path, err := exec.LookPath("docker")
	if err != nil {
		t.Fatalf("failed to find docker in path: %v", err)
	}
	return path
}
