package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
)

func reportStepStarted(ctx context.Context, data map[string]string) {
	if err := sendStepReport(ctx, data, "running", ""); err != nil {
		logrus.Errorf("failed to report upgrade started: %s", err.Error())
	}
}

func reportStepError(ctx context.Context, data map[string]string, errMsg string) {
	logrus.Error(errMsg)
	if err := sendStepReport(ctx, data, "failed", errMsg); err != nil {
		logrus.Errorf("failed to report upgrade error: %s", err.Error())
	}
}

func reportStepSuccess(ctx context.Context, data map[string]string) {
	if err := sendStepReport(ctx, data, "complete", ""); err != nil {
		logrus.Errorf("failed to report upgrade success: %s", err.Error())
	}
}

func sendStepReport(ctx context.Context, data map[string]string, status string, errMsg string) error {
	reportBody := map[string]string{
		"versionLabel": data["versionLabel"],
		"status":       status,
		"output":       "",
	}
	if errMsg != "" {
		reportBody["statusDescription"] = errMsg
	}

	body, err := json.Marshal(reportBody)
	if err != nil {
		return errors.Wrap(err, "marshal request body")
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}
	clusterIP, err := getKOTSClusterIP(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get kots cluster ip")
	}

	url := fmt.Sprintf("http://%s:3000/api/v1/app/%s/plan/%s", clusterIP, data["appSlug"], data["stepID"])
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
