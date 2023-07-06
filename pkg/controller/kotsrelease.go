package controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/replicatedhq/helmbin/pkg/config"
)

// KotsRelease implement the component interface to run the KotsRelease controller. This controller
// inspects for assets embed into the binary and apply them.
type KotsRelease struct {
	Options config.CLIOptions
	app     []byte
	log     logrus.FieldLogger
}

// Init initializes the KotsRelease controller. Reads the "sec_bundle" section of the elf binary if
// it exists. The "sec_bundle" section is expected to contain a tar.gz archive of the kots release
// yaml files. The embed process of this section does not happen here but it is something that is
// executed by the release build process.
func (k *KotsRelease) Init(_ context.Context) error {
	k.log = logrus.WithField("component", "kots-release")
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	fpbin, err := elf.Open(exe)
	if err != nil {
		return fmt.Errorf("failed to open %s binary: %w", os.Args[0], err)
	}
	defer fpbin.Close()
	section := fpbin.Section("sec_bundle")
	if section == nil {
		k.log.Infof("No kots release bundle found inside the binary.")
		return nil
	}
	k.log.Infof("Found embedded kots release bundle, inspecting it.")
	if k.app, err = k.processBundleSection(section); err != nil {
		return fmt.Errorf("failed to process bundle section: %w", err)
	}
	return nil
}

// processBundleSection reads the provided elf section. This section is expected to contain a tar.gz file
// with yamls inside, this function extracts the tar.gz content and searches for an Application CR yaml,
// if found returns its content. This function returns nil,nil if no Application CR is found.
func (k *KotsRelease) processBundleSection(section *elf.Section) ([]byte, error) {
	gzr, err := gzip.NewReader(section.Open())
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader for sec_bundle section: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil, nil
		case err != nil:
			return nil, fmt.Errorf("failed to read tar gz file: %w", err)
		case header == nil:
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content := bytes.NewBuffer(nil)
		if _, err := io.Copy(content, tr); err != nil {
			return nil, fmt.Errorf("failed to copy binary out of tar: %w", err)
		}
		if !bytes.Contains(content.Bytes(), []byte("apiVersion: kots.io/v1beta1")) {
			continue
		}
		if !bytes.Contains(content.Bytes(), []byte("kind: Application")) {
			continue
		}
		k.log.Infof("Kots Application definition found on file %s", header.Name)
		return content.Bytes(), nil
	}
}

// kubeclient builds a kubernetes client.
func (k *KotsRelease) kubeclient() (kubernetes.Interface, error) {
	cfg, err := kconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

// Start starts the KotsRelease controller. It creates or updates the kotsadm-application-metadata configmap in the
// default namespace and then finishes. If no kots release bundle is found inside the binary, this is a no-op.
func (k *KotsRelease) Start(ctx context.Context) error {
	if k.app == nil {
		k.log.Infof("No kots release bundle found, skipping")
		return nil
	}
	cli, err := k.kubeclient()
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}
	cm, err := cli.CoreV1().ConfigMaps("default").Get(ctx, "kotsadm-application-metadata", metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get kotsadm-application configmap: %w", err)
		}
		data := map[string]string{"application.yaml": string(k.app)}
		meta := metav1.ObjectMeta{Name: "kotsadm-application-metadata", Namespace: "default"}
		cm = &corev1.ConfigMap{ObjectMeta: meta, Data: data}
		if _, err := cli.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create kotsadm-application configmap: %w", err)
		}
		return nil
	}
	cm.Data["application.yaml"] = string(k.app)
	if _, err := cli.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update kotsadm-application configmap: %w", err)
	}
	return nil
}

// Stop stops the KotsRelease controller
func (k *KotsRelease) Stop() error {
	return nil
}
