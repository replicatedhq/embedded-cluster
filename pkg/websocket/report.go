package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func reportStepStarted(ctx context.Context, kcli client.Client, data map[string]string) {
	if err := sendStepReport(ctx, kcli, data, "running", ""); err != nil {
		logrus.Errorf("failed to report upgrade started: %s", err.Error())
	}
}

func reportStepFailed(ctx context.Context, kcli client.Client, data map[string]string, errMsg string) {
	logrus.Error(errMsg)
	if err := sendStepReport(ctx, kcli, data, "failed", errMsg); err != nil {
		logrus.Errorf("failed to report upgrade error: %s", err.Error())
	}
}

func reportStepComplete(ctx context.Context, kcli client.Client, data map[string]string) {
	if err := sendStepReport(ctx, kcli, data, "complete", ""); err != nil {
		logrus.Errorf("failed to report upgrade success: %s", err.Error())
	}
}

func sendStepReport(ctx context.Context, kcli client.Client, data map[string]string, status string, desc string) error {
	reportBody := map[string]string{
		"versionLabel": data["versionLabel"],
		"status":       status,
		"output":       "",
	}
	if desc != "" {
		reportBody["statusDescription"] = desc
	}

	body, err := json.Marshal(reportBody)
	if err != nil {
		return errors.Wrap(err, "marshal request body")
	}

	clusterIP, err := getKOTSClusterIP(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get kots cluster ip")
	}

	url := fmt.Sprintf("http://%s:%s/api/v1/app/%s/plan/%s", clusterIP, getKOTSPort(), data["appSlug"], data["stepID"])
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrap(err, "create request")
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("send request, status code: %d", resp.StatusCode)
	}
	return nil
}
