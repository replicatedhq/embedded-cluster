package socket

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
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

	return g.RunUnix(socketFile)
}

func GetSocketPath() (string, error) {
	// use tmp as it's only needed if the service is running
	tmpDir := os.TempDir()
	socketPath := filepath.Join(tmpDir, "ec.sock")

	return socketPath, nil
}
