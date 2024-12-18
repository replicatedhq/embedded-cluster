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
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/upgrade"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var wsDialer = &gwebsocket.Dialer{
	HandshakeTimeout: 10 * time.Second,
}

func ConnectToKOTSWebSocket(ctx context.Context) {
	for {
		if err := attemptConnection(ctx); err != nil {
			logrus.Errorf("Connection attempt to KOTS failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}
	}
}

func attemptConnection(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}
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

	wsURL := fmt.Sprintf("ws://%s:3000/ec-ws?nodeName=%s", clusterIP, node.Name)
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
	return listenToWSServer(ctx, conn)
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

func listenToWSServer(ctx context.Context, conn *gwebsocket.Conn) error {
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

		switch msg.Command {
		case "upgrade-cluster":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, d)

			var newInstall ecv1beta1.Installation
			if err := json.Unmarshal([]byte(d["installation"]), &newInstall); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal installation: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := upgrade.Upgrade(ctx, &newInstall); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to upgrade cluster: %s", err.Error()))
				continue
			}

			reportStepSuccess(ctx, d)

		case "add-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Add(ctx, repos, chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to add extension: %s", err.Error()))
				continue
			}

			reportStepSuccess(ctx, d)

		case "upgrade-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Upgrade(ctx, repos, chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to upgrade extension: %s", err.Error()))
				continue
			}

			reportStepSuccess(ctx, d)

		case "remove-extension":
			d := map[string]string{}
			if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
				logrus.Errorf("failed to unmarshal data: %s: %s", err, string(msg.Data))
				continue
			}

			reportStepStarted(ctx, d)

			var repos []k0sv1beta1.Repository
			if err := json.Unmarshal([]byte(d["repos"]), &repos); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal repos: %s: %s", err, string(msg.Data)))
				continue
			}

			var chart ecv1beta1.Chart
			if err := json.Unmarshal([]byte(d["chart"]), &chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to unmarshal chart: %s: %s", err, string(msg.Data)))
				continue
			}

			if err := extensions.Remove(ctx, repos, chart); err != nil {
				reportStepError(ctx, d, fmt.Sprintf("failed to remove extension: %s", err.Error()))
				continue
			}

			reportStepSuccess(ctx, d)
		default:
			logrus.Infof("Received unknown command: %s", msg.Command)
		}
	}
}
