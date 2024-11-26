package websocket

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	gwebsocket "github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
	var svc corev1.Service
	if err := kcli.Get(ctx, types.NamespacedName{Name: "kotsadm", Namespace: runtimeconfig.KotsadmNamespace}, &svc); err != nil {
		return errors.Wrap(err, "get kotsadm service")
	}
	clusterIP := svc.Spec.ClusterIP
	if clusterIP == "" {
		return errors.New("cluster ip is empty")
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

	dialer := gwebsocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("connect to websocket server: %w", err)
	}
	defer conn.Close()

	logrus.Info("Successfully connected to KOTS WebSocket server")

	// ping server on a regular interval to make sure it's still connected
	go pingWSServer(conn)

	// listen to server messages
	return listenToWSServer(conn)
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

func listenToWSServer(conn *gwebsocket.Conn) error {
	for {
		_, _, err := conn.ReadMessage() // this is required to receive ping/pong messages
		if err != nil {
			return errors.Wrap(err, "read message")
		}
	}
}
