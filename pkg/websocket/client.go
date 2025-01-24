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
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/embedded-cluster/pkg/websocket/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
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
	endpoint, err := getKOTSEndpoint(ctx, kcli)
	if err != nil {
		return errors.Wrap(err, "get kots endpoint")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "get hostname")
	}
	var node corev1.Node
	if err := kcli.Get(ctx, k8stypes.NamespacedName{Name: hostname}, &node); err != nil {
		return errors.Wrap(err, "get node")
	}

	wsURL := fmt.Sprintf("ws://%s/ec-ws?nodeName=%s&version=%s", endpoint, url.QueryEscape(node.Name), url.QueryEscape(versions.Version))
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
	go pingWSServer(ctx, conn)

	// listen to server messages
	return listenToWSServer(ctx, conn, endpoint)
}

func pingWSServer(ctx context.Context, conn *gwebsocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * time.Duration(5+rand.Intn(16))): // 5-20 seconds
			pingMsg := fmt.Sprintf("%x", rand.Int())
			if err := conn.WriteControl(gwebsocket.PingMessage, []byte(pingMsg), time.Now().Add(1*time.Second)); err != nil {
				return errors.Wrap(err, "send ping message")
			}
		}
	}
}

func listenToWSServer(ctx context.Context, conn *gwebsocket.Conn, endpoint string) error {
	for {
		_, message, err := conn.ReadMessage() // receive messages, including ping/pong
		if err != nil {
			return errors.Wrap(err, "read message")
		}

		var msg types.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logrus.Errorf("failed to unmarshal message: %s: %s", err, string(message))
			continue
		}
		if err := msg.Validate(); err != nil {
			logrus.Errorf("invalid message: %s", err.Error())
			continue
		}

		logrus.Infof("Processing message with command=%s app=%s version=%s step=%s", msg.Command, msg.AppSlug, msg.VersionLabel, msg.StepID)

		// ensure the environment is set up correctly
		os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
		os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

		// create a reporter for this step
		stepReporter := NewStepReporter(ctx, endpoint, msg.AppSlug, msg.VersionLabel, msg.StepID)
		stepReporter.Started()

		if err := processWSMessage(ctx, msg); err != nil {
			logrus.Errorf("failed to process message: %s", err.Error())
			stepReporter.Failed(fmt.Sprintf("failed to process message: %s", err.Error()))
			continue
		}

		// this doesn't get called for the upgrade manager step because the manager restarts.
		// kots marks the step as complete when the new manager connects to it.
		logrus.Infof("Successfully proccessed message with command=%s app=%s version=%s step=%s", msg.Command, msg.AppSlug, msg.VersionLabel, msg.StepID)
		stepReporter.Complete()
	}
}

func processWSMessage(ctx context.Context, msg types.Message) error {
	switch msg.Command {
	case types.CommandUpgradeManager:
		d := types.UpgradeManagerData{}
		if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
			return errors.Wrapf(err, "unmarshal data: %s", string(msg.Data))
		}
		if err := d.Validate(); err != nil {
			return errors.Wrap(err, "invalid data")
		}

		if err := manager.Upgrade(ctx, manager.UpgradeOptions{
			AppSlug:         msg.AppSlug,
			VersionLabel:    msg.VersionLabel,
			LicenseID:       d.LicenseID,
			LicenseEndpoint: d.LicenseEndpoint,
		}); err != nil {
			return errors.Wrap(err, "upgrade manager")
		}

	case types.CommandUpgradeCluster:
		d := types.UpgradeClusterData{}
		if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
			return errors.Wrapf(err, "unmarshal data: %s", string(msg.Data))
		}
		if err := d.Validate(); err != nil {
			return errors.Wrap(err, "invalid data")
		}
		if err := upgrade.Upgrade(ctx, &d.Installation); err != nil {
			return errors.Wrap(err, "upgrade cluster")
		}

	case types.CommandAddExtension:
		d := types.ExtensionData{}
		if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
			return errors.Wrapf(err, "unmarshal data: %s", string(msg.Data))
		}
		if err := extensions.Add(ctx, d.Repos, d.Chart); err != nil {
			return errors.Wrap(err, "add extension")
		}

	case types.CommandUpgradeExtension:
		d := types.ExtensionData{}
		if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
			return errors.Wrapf(err, "unmarshal data: %s", string(msg.Data))
		}
		if err := extensions.Upgrade(ctx, d.Repos, d.Chart); err != nil {
			return errors.Wrap(err, "upgrade extension")
		}

	case types.CommandRemoveExtension:
		d := types.ExtensionData{}
		if err := json.Unmarshal([]byte(msg.Data), &d); err != nil {
			return errors.Wrapf(err, "unmarshal data: %s", string(msg.Data))
		}
		if err := extensions.Remove(ctx, d.Repos, d.Chart); err != nil {
			return errors.Wrap(err, "remove extension")
		}
	default:
		return errors.Errorf("unknown command: %s", msg.Command)
	}

	return nil
}
