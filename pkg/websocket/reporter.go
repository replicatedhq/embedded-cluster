package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type StepReporter struct {
	ctx          context.Context
	endpoint     string
	appSlug      string
	versionLabel string
	stepID       string
}

func NewStepReporter(ctx context.Context, endpoint, appSlug, versionLabel, stepID string) *StepReporter {
	return &StepReporter{
		ctx:          ctx,
		endpoint:     endpoint,
		appSlug:      appSlug,
		versionLabel: versionLabel,
		stepID:       stepID,
	}
}

func (r *StepReporter) Started() {
	if err := r.sendReport("running", ""); err != nil {
		logrus.Errorf("failed to report step started: %s", err.Error())
	}
}

func (r *StepReporter) Failed(errMsg string) {
	logrus.Error(errMsg)
	if err := r.sendReport("failed", errMsg); err != nil {
		logrus.Errorf("failed to report step error: %s", err.Error())
	}
}

func (r *StepReporter) Complete() {
	if err := r.sendReport("complete", ""); err != nil {
		logrus.Errorf("failed to report step success: %s", err.Error())
	}
}

func (r *StepReporter) sendReport(status string, desc string) error {
	reportBody := map[string]string{
		"versionLabel": r.versionLabel,
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

	url := fmt.Sprintf("http://%s/api/v1/app/%s/plan/%s", r.endpoint, r.appSlug, r.stepID)
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
