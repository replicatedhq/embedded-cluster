package socket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/upgrade"
	"gopkg.in/yaml.v2"
)

func StartSocketServer(ctx context.Context) error {
	socketFile, err := GetSocketPath()
	if err != nil {
		return fmt.Errorf("get socket path: %w", err)
	}

	if err := os.Remove(socketFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove socket file: %w", err)
	}

	g := gin.Default()
	g.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	g.POST("/upgradecluster", func(c *gin.Context) {
		// read the yaml from the body
		installationYAML, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		// unmarshal the yaml into a clusterv1beta1.Installation object
		var installation clusterv1beta1.Installation
		if err := yaml.Unmarshal(installationYAML, &installation); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		if err := upgrade.Upgrade(ctx, &installation); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	return g.RunUnix(socketFile)
}

func GetSocketPath() (string, error) {
	// use tmp (in the EC data dir) as it's only needed if the service is running
	tmpDir := os.TempDir()
	socketPath := filepath.Join(tmpDir, "ec.sock")

	return socketPath, nil
}
