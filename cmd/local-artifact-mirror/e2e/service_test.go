package e2e

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	lamServiceURL = "http://localhost:50000"
)

type lamTest struct {
	t      *testing.T
	tmpDir string
	cmd    *exec.Cmd
}

// setupService starts the LAM service in the background
func setupService(t *testing.T) *lamTest {
	// make a tmp dir for data-dir for tests
	tmpDir, err := os.MkdirTemp("", "lam-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(tmpDir+"/bin", 0755); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/charts", 0755); err != nil {
		t.Fatalf("Failed to create charts directory: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/images", 0755); err != nil {
		t.Fatalf("Failed to create images directory: %v", err)
	}

	copyTestDataToTmpDir := func(t *testing.T) {
		subDirs := []string{"charts", "images"}
		for _, subDir := range subDirs {
			cmd := exec.Command("cp", "-r", "./testdata/"+subDir+"/", filepath.Join(tmpDir, subDir))

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("Failed to capture stdout: %v", err)
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatalf("Failed to capture stderr: %v", err)
			}

			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			if _, err := io.Copy(os.Stdout, stdout); err != nil {
				t.Fatalf("Failed to copy stdout: %v", err)
			}
			if _, err := io.Copy(os.Stderr, stderr); err != nil {
				t.Fatalf("Failed to copy stderr: %v", err)
			}

			if err := cmd.Wait(); err != nil {
				t.Fatalf("Failed to wait for command: %v", err)
			}
		}
	}
	copyTestDataToTmpDir(t)

	copyBinToTmpDir := func(t *testing.T) {
		cmd := exec.Command("cp", "../../../pkg/goods/bins/local-artifact-mirror", filepath.Join(tmpDir, "bin"))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("Failed to capture stdout: %v", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Fatalf("Failed to capture stderr: %v", err)
		}

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start LAM service: %v", err)
		}

		go func() {
			io.Copy(log.Writer(), stdout)
		}()
		go func() {
			io.Copy(log.Writer(), stderr)
		}()
	}
	copyBinToTmpDir(t)

	cmd := exec.Command("../../../pkg/goods/bins/local-artifact-mirror", "serve", "--port", "50000", "--data-dir", tmpDir)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start LAM service: %v", err)
	}

	go func() {
		io.Copy(log.Writer(), stdout)
	}()
	go func() {
		io.Copy(log.Writer(), stderr)
	}()

	time.Sleep(2 * time.Second)

	return &lamTest{
		t:      t,
		tmpDir: tmpDir,
		cmd:    cmd,
	}
}

func (lt *lamTest) teardownService() {
	if lt.cmd != nil && lt.cmd.Process != nil {
		lt.cmd.Process.Kill()
	}
	os.RemoveAll(lt.tmpDir) // Clean up the temp directory after the test
}

// TestServiceStart checks if the LAM service starts successfully and responds correctly
func TestServiceStart(t *testing.T) {
	lt := setupService(t)
	defer lt.teardownService()

	resp, err := http.Get(lamServiceURL)
	if err != nil {
		t.Fatalf("Failed to reach LAM service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status Not Found, got %v", resp.Status)
	}
}

// TestFetchArtifact verifies the service can serve a sample artifact
func TestFetchArtifact(t *testing.T) {
	lt := setupService(t)
	defer lt.teardownService()

	artifactPath := lamServiceURL + "/images/sample-artifact.tar.gz"
	resp, err := http.Get(artifactPath)
	if err != nil {
		t.Fatalf("Failed to fetch artifact: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for artifact fetch, got %v", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil || len(content) == 0 {
		t.Error("Failed to read artifact content or content is empty")
	}
}
