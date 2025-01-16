package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	gwebsocket "github.com/gorilla/websocket"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var wsDialer = &gwebsocket.Dialer{
	HandshakeTimeout: 10 * time.Second,
}

func ConnectToKOTSWebSocket(ctx context.Context, kcli client.Client) {
	for {
		if err := attemptConnection(ctx, kcli); err != nil {
			logrus.Errorf("Connection attempt to KOTS failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}
	}
}

func attemptConnection(ctx context.Context, kcli client.Client) error {
	clusterIP, err := getKOTSClusterIP(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get kots cluster ip")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "get hostname")
	}
	var node corev1.Node
	if err := kcli.Get(ctx, types.NamespacedName{Name: hostname}, &node); err != nil {
		return errors.Wrap(err, "get node")
	}

	wsURL := fmt.Sprintf("ws://%s:%s/ec-ws?nodeName=%s&version=%s", clusterIP, getKOTSPort(), url.QueryEscape(node.Name), url.QueryEscape(versions.Version))
	logrus.Infof("connecting to KOTS WebSocket server on %s", wsURL)
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("parse websocket url: %w", err)
	}

	conn, _, err := wsDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("connect to websocket server: %w", err)
	}
	defer conn.Close()

	logrus.Info("Successfully connected to KOTS WebSocket server")

	// ping server on a regular interval to make sure it's still connected
	go pingWSServer(conn)

	// listen to server messages
	return listenToWSServer(ctx, conn, kcli)
}

func pingWSServer(conn *gwebsocket.Conn) error {
	for {
		sleepDuration := time.Second * time.Duration(5+rand.Intn(16)) // 5-20 seconds
		time.Sleep(sleepDuration)

		pingMsg := fmt.Sprintf("%x", rand.Int())

		if err := conn.WriteControl(gwebsocket.PingMessage, []byte(pingMsg), time.Now().Add(1*time.Second)); err != nil {
			return errors.Wrap(err, "send ping message")
		}
	}
}

type Message struct {
	Command string `json:"command"`
	Data    string `json:"data"`
}

func listenToWSServer(ctx context.Context, conn *gwebsocket.Conn, kcli client.Client) error {
	for {
		_, message, err := conn.ReadMessage() // receive messages, including ping/pong
		if err != nil {
			return errors.Wrap(err, "read message")
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logrus.Errorf("failed to unmarshal message: %s: %s", err, string(message))
			continue
		}

		// ensure the environment is set up correctly
		os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
		os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

		switch msg.Command {
		case "upgrade-manager":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, kcli, d)

			// path to the manager binary on the host
			binPath := runtimeconfig.PathToEmbeddedClusterBinary("manager")

			// TODO (@salah): airgap
			err := manager.DownloadBinaryOnline(ctx, binPath, d["licenseID"], d["licenseEndpoint"], d["versionLabel"])
			if err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to download manager binary: %s", err))
				continue
			}

			// this is hacky but app slug is what determines the service name
			manager.SetServiceName(d["appSlug"])

			if err := systemd.Restart(ctx, manager.UnitName()); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to restart manager service: %s", err))
				continue
			}

			// kots marks the step as complete when the new manager connects to it
			// TODO (@salah): figure out a better way to do this ^

		case "upgrade-cluster":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, kcli, d)

			var newInstall ecv1beta1.Installation
			if err := json.Unmarshal([]byte(d["installation"]), &newInstall); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal installation: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := upgrade.Upgrade(ctx, &newInstall); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to upgrade cluster: %s", err.Error()))
				continue
			}

			reportStepComplete(ctx, kcli, d)

		case "add-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, kcli, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Add(ctx, repos, chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to add extension: %s", err.Error()))
				continue
			}

			reportStepComplete(ctx, kcli, d)

		case "upgrade-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, kcli, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Upgrade(ctx, repos, chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to upgrade extension: %s", err.Error()))
				continue
			}

			reportStepComplete(ctx, kcli, d)

		case "remove-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, kcli, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Remove(ctx, repos, chart); err != nil {
				reportStepFailed(ctx, kcli, d, fmt.Sprintf("failed to remove extension: %s", err.Error()))
				continue
			}

			reportStepComplete(ctx, kcli, d)
		default:
			logrus.Infof("Received unknown command: %s", msg.Command)
		}
	}
}
