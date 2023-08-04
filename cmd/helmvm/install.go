package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/log"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"

	"github.com/replicatedhq/helmvm/pkg/addons"
	"github.com/replicatedhq/helmvm/pkg/config"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
	"github.com/replicatedhq/helmvm/pkg/infra"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

// runPostApply is meant to run things that can't be run automatically with
// k0sctl. Iterates over all hosts and calls runPostApply on each.
func runPostApply(ctx context.Context) error {
	logrus.Infof("Running post-apply script on nodes")
	logger, end := pb.Start()
	orig := log.Log
	rig.SetLogger(logger)
	defer func() {
		logger.Infof("post apply process finished")
		close(logger)
		<-end
		log.Log = orig
	}()
	cfg, err := config.ReadConfigFile(defaults.PathToConfig("k0sctl.yaml"))
	if err != nil {
		return fmt.Errorf("unable to read cluster config: %w", err)
	}
	for _, host := range cfg.Spec.Hosts {
		if err := runPostApplyOnHost(ctx, host, logger); err != nil {
			return err
		}
	}
	return nil
}

// runPostApply runs the post-apply script on a host. XXX I don't think this
// belongs here and needs to be refactored in a more generic way. It's here
// because I have other things to do and this is a prototype.
func runPostApplyOnHost(ctx context.Context, host *cluster.Host, logger pb.MessageWriter) error {
	if err := host.Connect(); err != nil {
		return fmt.Errorf("failed to connect to host: %w", err)
	}
	src := "/etc/systemd/system/k0scontroller.service"
	if host.Role == "worker" {
		src = "/etc/systemd/system/k0sworker.service"
	}
	dst := fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
	host.ExecOutput(fmt.Sprintf("sudo ln -s %s %s", src, dst))
	host.ExecOutput("sudo systemctl daemon-reload")
	return nil
}

// createK0sctlConfigBackup creates a backup of the current k0sctl.yaml file.
func createK0sctlConfigBackup(ctx context.Context) error {
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	if _, err := os.Stat(cfgpath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to stat config: %w", err)
	}
	bkdir := path.Join(defaults.ConfigSubDir(), "backup")
	if err := os.MkdirAll(bkdir, 0700); err != nil {
		return fmt.Errorf("unable to create backup dir: %w", err)
	}
	fname := fmt.Sprintf("k0sctl.yaml-%d", time.Now().UnixNano())
	bakpath := path.Join(bkdir, fname)
	logrus.Infof("Creating k0sctl.yaml backup in %s", bkdir)
	data, err := os.ReadFile(cfgpath)
	if err != nil {
		return fmt.Errorf("unable to read config: %w", err)
	}
	if err := os.WriteFile(bakpath, data, 0600); err != nil {
		return fmt.Errorf("unable to write config backup: %w", err)
	}
	return nil
}

// updateConfigBundle updates the k0sctl.yaml file in the configuration directory
// to use the bundle in the specified directory (reads the bundle directory and
// updates the files that need to be uploaded to the nodes).
func updateConfigBundle(ctx context.Context, bundledir string) error {
	if err := createK0sctlConfigBackup(ctx); err != nil {
		return fmt.Errorf("unable to create config backup: %w", err)
	}
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	cfg, err := config.ReadConfigFile(cfgpath)
	if err != nil {
		return fmt.Errorf("unable to read cluster config: %w", err)
	}
	if err := config.UpdateHostsFiles(cfg, bundledir); err != nil {
		return fmt.Errorf("unable to update hosts files: %w", err)
	}
	fp, err := os.OpenFile(cfgpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer fp.Close()
	if err := yaml.NewEncoder(fp).Encode(cfg); err != nil {
		return fmt.Errorf("unable to write config file: %w", err)
	}
	return nil
}

// copyUserProvidedConfig copies the user provided configuration to the config directory.
func copyUserProvidedConfig(c *cli.Context) error {
	usercfg := c.String("config")
	cfg, err := config.ReadConfigFile(usercfg)
	if err != nil {
		return fmt.Errorf("unable to read cluster config: %w", err)
	}
	bundledir := c.String("bundle")
	if err := config.UpdateHostsFiles(cfg, bundledir); err != nil {
		return fmt.Errorf("unable to update hosts files: %w", err)
	}
	if err := createK0sctlConfigBackup(c.Context); err != nil {
		return fmt.Errorf("unable to create config backup: %w", err)
	}
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	fp, err := os.OpenFile(cfgpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer fp.Close()
	if err := yaml.NewEncoder(fp).Encode(cfg); err != nil {
		return fmt.Errorf("unable to write config file: %w", err)
	}
	return nil
}

// ensureK0sctlConfig ensures that a k0sctl.yaml file exists in the configuration
// directory. If none exists then this directs the user to a wizard to create one.
func ensureK0sctlConfig(c *cli.Context, nodes []infra.Node) error {
	bundledir := c.String("bundle-dir")
	bundledir = strings.TrimRight(bundledir, "/")
	multi := c.Bool("multi-node")
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	if usercfg := c.String("config"); usercfg != "" {
		logrus.Infof("Using %s config file", usercfg)
		return copyUserProvidedConfig(c)
	}
	var useCurrent = &survey.Confirm{
		Message: "Do you want to use the existing configuration ?",
		Default: true,
	}
	if _, err := os.Stat(cfgpath); err == nil {
		var answer bool
		logrus.Warn("A cluster configuration file was found. This means you already")
		logrus.Warn("have created a cluster configured. You can either use the existing")
		logrus.Warn("configuration or create a new one (the original configuration will")
		logrus.Warn("be backed up).")
		if err := survey.AskOne(useCurrent, &answer); err != nil {
			return fmt.Errorf("unable to process answers: %w", err)
		} else if answer {
			return updateConfigBundle(c.Context, bundledir)
		}
		if err := createK0sctlConfigBackup(c.Context); err != nil {
			return fmt.Errorf("unable to create config backup: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("unable to open config: %w", err)
	}
	if !multi && runtime.GOOS != "linux" {
		return fmt.Errorf("single node clusters only supported on linux")
	}
	cfg, err := config.RenderClusterConfig(c.Context, nodes, multi)
	if err != nil {
		return fmt.Errorf("unable to render config: %w", err)
	}
	if err := config.UpdateHostsFiles(cfg, bundledir); err != nil {
		return fmt.Errorf("unable to update hosts files: %w", err)
	}
	fp, err := os.OpenFile(cfgpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer fp.Close()
	if err := yaml.NewEncoder(fp).Encode(cfg); err != nil {
		return fmt.Errorf("unable to write config file: %w", err)
	}
	return nil
}

// runK0sctlApply runs `k0sctl apply` refering to the k0sctl.yaml file found on
// the configuration directory. Returns when the command is finished.
func runK0sctlApply(ctx context.Context) error {
	bin := defaults.PathToHelmVMBinary("k0sctl")
	messages, pbwait := pb.Start()
	defer func() {
		messages.Close()
		<-pbwait
	}()
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	messages.Write([]byte("Running k0sctl apply"))
	kctl := exec.Command(bin, "apply", "-c", cfgpath, "--disable-telemetry")
	kctl.Stderr = messages
	kctl.Stdout = messages
	return kctl.Run()
}

// runK0sctlKubeconfig runs the `k0sctl kubeconfig` command. Result is saved
// under a file called "kubeconfig" inside defaults.ConfigSubDir(). XXX File
// is overwritten, no questions asked.
func runK0sctlKubeconfig(ctx context.Context) error {
	bin := defaults.PathToHelmVMBinary("k0sctl")
	kpath := defaults.PathToConfig("kubeconfig")
	fp, err := os.OpenFile(kpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open kubeconfig: %w", err)
	}
	defer fp.Close()
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	kctl := exec.Command(bin, "kubeconfig", "-c", cfgpath, "--disable-telemetry")
	kctl.Stderr = fp
	kctl.Stdout = fp
	if err := kctl.Run(); err != nil {
		return fmt.Errorf("unable to run kubeconfig: %w", err)
	}
	logrus.Infof("Kubeconfig saved to %s", kpath)
	return nil
}

// dumpApplyLogs dumps all k0sctl apply command output to the stdout.
func dumpApplyLogs() {
	logs := defaults.K0sctlApplyLogPath()
	fp, err := os.Open(logs)
	if err != nil {
		logrus.Errorf("Unable to read logs: %s", err.Error())
		return
	}
	defer fp.Close()
	_, _ = io.Copy(os.Stdout, fp)
}

// applyK0sctl runs the k0sctl apply command and waits for it to finish. If
// no configuration is found one is generated.
func applyK0sctl(c *cli.Context, nodes []infra.Node) error {
	logrus.Infof("Processing cluster configuration")
	if err := ensureK0sctlConfig(c, nodes); err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	logrus.Infof("Applying cluster configuration")
	if err := runK0sctlApply(c.Context); err != nil {
		logrus.Errorf("Installation or upgrade failed.")
		var useCurrent = &survey.Confirm{
			Message: "Do you wish to visualize the logs?",
			Default: true,
		}
		var answer bool
		if err := survey.AskOne(useCurrent, &answer); err != nil {
			return fmt.Errorf("unable to process answers: %w", err)
		} else if answer {
			dumpApplyLogs()
		}
		return fmt.Errorf("unable to apply cluster: %w", err)
	}
	return nil
}

// installCommands executes the "install" command. This will ensure that a
// k0sctl.yaml file exists and then run `k0sctl apply` to apply the cluster.
// Once this is finished then a "kubeconfig" file is created and the addons
// are applied. Resulting k0sctl.yaml and kubeconfig are stored in the
// configuration dir.
var installCommand = &cli.Command{
	Name:    "install",
	Aliases: []string{"apply"},
	Usage:   "Installs a new or upgrades an existing cluster",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "bundle-dir",
			Usage: "Disconnected environment bundle path",
		},
		&cli.StringFlag{
			Name:  "config",
			Usage: "Path to the configuration to be applied",
		},
		&cli.StringFlag{
			Name:  "infra",
			Usage: "Path to a directory with terraform infra manifests",
		},
		&cli.BoolFlag{
			Name:  "multi-node",
			Usage: "Installs or upgrades a multi node deployment",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "addons-only",
			Usage: "Only apply addons. Skips cluster install",
			Value: false,
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("Materializing binaries")
		if err := goods.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize binaries: %w", err)
		}
		if !c.Bool("addons-only") {
			var err error
			var nodes []infra.Node
			if dir := c.String("infra"); dir != "" {
				logrus.Infof("Processing infrastructure manifests")
				if nodes, err = infra.Apply(c.Context, dir); err != nil {
					return fmt.Errorf("unable to create infra: %w", err)
				}
			}
			if err := applyK0sctl(c, nodes); err != nil {
				return fmt.Errorf("unable update cluster: %w", err)
			}
		}
		logrus.Infof("Reading cluster access configuration")
		if err := runK0sctlKubeconfig(c.Context); err != nil {
			return fmt.Errorf("unable to get kubeconfig: %w", err)
		}
		logrus.Infof("Applying add-ons")
		ccfg := defaults.PathToConfig("k0sctl.yaml")
		kcfg := defaults.PathToConfig("kubeconfig")
		os.Setenv("KUBECONFIG", kcfg)
		if applier, err := addons.NewApplier(); err != nil {
			return fmt.Errorf("unable to create applier: %w", err)
		} else if err := applier.Apply(c.Context); err != nil {
			return fmt.Errorf("unable to apply addons: %w", err)
		}
		if err := runPostApply(c.Context); err != nil {
			return fmt.Errorf("unable to run post apply: %w", err)
		}
		logrus.Infof("Cluster configuration has been applied")
		logrus.Infof("Kubeconfig file has been placed at at %s", kcfg)
		logrus.Infof("Cluster configuration file has been placed at %s", ccfg)
		logrus.Infof("You can now access your cluster with kubectl by running:")
		logrus.Infof("  %s shell", os.Args[0])
		return nil
	},
}
