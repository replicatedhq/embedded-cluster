package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

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
	"github.com/replicatedhq/helmvm/pkg/metrics"
	"github.com/replicatedhq/helmvm/pkg/preflights"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

// runPostApply is meant to run things that can't be run automatically with
// k0sctl. Iterates over all hosts and calls runPostApply on each.
func runPostApply(ctx context.Context) error {
	mask := func(raw string) string {
		logrus.StandardLogger().Writer().Write([]byte(raw))
		return fmt.Sprintf("Creating systemd unit for %s", defaults.BinaryName())
	}
	loading := pb.Start(pb.WithMask(mask))
	orig := log.Log
	rig.SetLogger(loading)
	defer func() {
		loading.Close()
		log.Log = orig
	}()
	cfg, err := config.ReadConfigFile(defaults.PathToConfig("k0sctl.yaml"))
	if err != nil {
		return fmt.Errorf("unable to read cluster config: %w", err)
	}
	for _, host := range cfg.Spec.Hosts {
		if err := runPostApplyOnHost(ctx, host); err != nil {
			return err
		}
	}
	return nil
}

// runHostPreflights run the host preflights we found embedded in the binary
// on all configured hosts. We attempt to read HostPreflights from all the
// embedded Helm Charts and from the Kots Application Release files.
func runHostPreflights(c *cli.Context) error {
	logrus.Infof("Running host preflights on nodes")
	cfg, err := config.ReadConfigFile(defaults.PathToConfig("k0sctl.yaml"))
	if err != nil {
		return fmt.Errorf("unable to read cluster config: %w", err)
	}
	hpf, err := addons.NewApplier().HostPreflights()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		logrus.Info("No host preflights found")
		return nil
	}
	outputs := preflights.NewOutputs()
	for _, host := range cfg.Spec.Hosts {
		addr := host.Address()
		out, err := preflights.Run(c.Context, host, hpf)
		if err != nil {
			return fmt.Errorf("preflight failed on %s: %w", addr, err)
		}
		outputs[addr] = out
	}
	outputs.PrintTable()
	if outputs.HaveFails() {
		return fmt.Errorf("preflights haven't passed on one or more hosts")
	}
	if !outputs.HaveWarns() || c.Bool("no-prompt") {
		return nil
	}
	fmt.Println("Host preflights have warnings on one or more hosts")
	if !prompts.New().Confirm("Do you want to continue ?", false) {
		return fmt.Errorf("user aborted")
	}
	return nil
}

// runPostApply runs the post-apply script on a host. XXX I don't think this
// belongs here and needs to be refactored in a more generic way. It's here
// because I have other things to do and this is a prototype.
func runPostApplyOnHost(ctx context.Context, host *cluster.Host) error {
	if err := host.Connect(); err != nil {
		return fmt.Errorf("failed to connect to host: %w", err)
	}
	defer host.Disconnect()
	src := "/etc/systemd/system/k0scontroller.service"
	if host.Role == "worker" {
		src = "/etc/systemd/system/k0sworker.service"
	}
	dst := fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
	_, _ = host.ExecOutput(fmt.Sprintf("sudo ln -s %s %s", src, dst))
	_, _ = host.ExecOutput("sudo systemctl daemon-reload")
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
// updates the files that need to be uploaded to the nodes). This function also
// makes sure that the k0s version used in the configuration matches the version
// we are planning to install.
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
	cfg.Spec.K0s.Version = defaults.K0sVersion
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

// copyUserProvidedConfig copies the user provided configuration to the config dir.
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

// overwriteExistingConfig asks user if they want to overwrite the existing cluster
// configuration file.
func overwriteExistingConfig() bool {
	fmt.Println("A cluster configuration file was found. This means you already")
	fmt.Println("have created and configured a cluster. You can either use the")
	fmt.Println("existing configuration or create a new one (the original config")
	fmt.Println("will be backed up).")
	return prompts.New().Confirm(
		"Do you want to create a new cluster configuration ?", false,
	)
}

// ensureK0sctlConfig ensures that a k0sctl.yaml file exists in the configuration
// directory. If none exists then this directs the user to a wizard to create one.
func ensureK0sctlConfig(c *cli.Context, nodes []infra.Node, useprompt bool) error {
	multi := c.Bool("multi-node") || len(nodes) > 0
	if !multi && runtime.GOOS != "linux" {
		return fmt.Errorf("single node clusters only supported on linux")
	}
	bundledir := c.String("bundle-dir")
	bundledir = strings.TrimRight(bundledir, "/")
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	if usercfg := c.String("config"); usercfg != "" {
		logrus.Infof("Using %s config file", usercfg)
		return copyUserProvidedConfig(c)
	}
	if _, err := os.Stat(cfgpath); err == nil {
		if len(nodes) == 0 {
			if !useprompt {
				return updateConfigBundle(c.Context, bundledir)
			}
			if !overwriteExistingConfig() {
				return updateConfigBundle(c.Context, bundledir)
			}
		}
		if err := createK0sctlConfigBackup(c.Context); err != nil {
			return fmt.Errorf("unable to create config backup: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("unable to open config: %w", err)
	}
	cfg, err := config.RenderClusterConfig(c.Context, nodes, multi)
	if err != nil {
		return fmt.Errorf("unable to render config: %w", err)
	}
	if bundledir != "" {
		config.SetUploadBinary(cfg)
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
	message := "Applying cluster configuration"
	mask := func(raw string) string {
		logrus.StandardLogger().Writer().Write([]byte(raw))
		if !strings.Contains(raw, "Running phase:") {
			return message
		}
		slices := strings.SplitN(raw, ":", 2)
		message = strings.ReplaceAll(slices[1], `"`, "")
		message = strings.TrimSpace(message)
		message = strings.ReplaceAll(message, "k0s", defaults.BinaryName())
		message = strings.ReplaceAll(message, "Upload", "Copy")
		message = fmt.Sprintf("Phase: %s", message)
		return message
	}
	bin := defaults.PathToHelmVMBinary("k0sctl")
	loading := pb.Start(pb.WithMask(mask))
	defer func() {
		loading.Closef("Finished applying cluster configuration")
	}()
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	kctl := exec.Command(bin, "apply", "-c", cfgpath, "--disable-telemetry")
	kctl.Stderr = loading
	kctl.Stdout = loading
	return kctl.Run()
}

// runK0sctlKubeconfig runs the `k0sctl kubeconfig` command. Result is saved
// under a file called "kubeconfig" inside defaults.ConfigSubDir(). XXX File
// is overwritten, no questions asked.
func runK0sctlKubeconfig(ctx context.Context) error {
	bin := defaults.PathToHelmVMBinary("k0sctl")
	cfgpath := defaults.PathToConfig("k0sctl.yaml")
	if _, err := os.Stat(cfgpath); err != nil {
		return fmt.Errorf("cluster configuration not found")
	}
	buf := bytes.NewBuffer(nil)
	kctl := exec.Command(bin, "kubeconfig", "-c", cfgpath)
	kctl.Stderr, kctl.Stdout = buf, buf
	if err := kctl.Run(); err != nil {
		logrus.Errorf("Failed to read kubeconfig:")
		logrus.Errorf(buf.String())
		return fmt.Errorf("unable to run kubeconfig: %w", err)
	}
	kpath := defaults.PathToConfig("kubeconfig")
	fp, err := os.OpenFile(kpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open kubeconfig: %w", err)
	}
	defer fp.Close()
	if _, err := io.Copy(fp, buf); err != nil {
		return fmt.Errorf("unable to write kubeconfig: %w", err)
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
func applyK0sctl(c *cli.Context, useprompt bool, nodes []infra.Node) error {
	fmt.Println("Processing cluster configuration")
	if err := ensureK0sctlConfig(c, nodes, useprompt); err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	if err := runHostPreflights(c); err != nil {
		return fmt.Errorf("unable to finish preflight checks: %w", err)
	}
	fmt.Println("Applying cluster configuration")
	if err := runK0sctlApply(c.Context); err != nil {
		logrus.Errorf("Installation or upgrade failed.")
		if !useprompt {
			dumpApplyLogs()
			return fmt.Errorf("unable to apply cluster: %w", err)
		}
		msg := "Do you wish to visualize the logs?"
		if prompts.New().Confirm(msg, true) {
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
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
		&cli.StringSliceFlag{
			Name:  "disable-addon",
			Usage: "Disable addon during install/upgrade",
		},
	},
	Action: func(c *cli.Context) error {
		metrics.ReportApplyStarted(c)
		if defaults.DecentralizedInstall() {
			fmt.Println("Decentralized install was detected. To manage the cluster")
			fmt.Printf("you have to use the '%s node' commands instead.\n", defaults.BinaryName())
			fmt.Printf("Run '%s node --help' for more information.\n", defaults.BinaryName())
			metrics.ReportApplyFinished(c, fmt.Errorf("wrong upgrade on decentralized install"))
			return fmt.Errorf("decentralized install detected")
		}
		useprompt := !c.Bool("no-prompt")
		logrus.Infof("Materializing binaries")
		if err := goods.Materialize(); err != nil {
			err := fmt.Errorf("unable to materialize binaries: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		if !c.Bool("addons-only") {
			var err error
			var nodes []infra.Node
			if dir := c.String("infra"); dir != "" {
				logrus.Infof("Processing infrastructure manifests")
				if nodes, err = infra.Apply(c.Context, dir, useprompt); err != nil {
					err := fmt.Errorf("unable to create infra: %w", err)
					metrics.ReportApplyFinished(c, err)
					return err
				}
			}
			if err := applyK0sctl(c, useprompt, nodes); err != nil {
				err := fmt.Errorf("unable update cluster: %w", err)
				metrics.ReportApplyFinished(c, err)
				return err
			}
		}
		logrus.Infof("Reading cluster access configuration")
		if err := runK0sctlKubeconfig(c.Context); err != nil {
			err := fmt.Errorf("unable to get kubeconfig: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Infof("Applying add-ons")
		ccfg := defaults.PathToConfig("k0sctl.yaml")
		kcfg := defaults.PathToConfig("kubeconfig")
		os.Setenv("KUBECONFIG", kcfg)
		opts := []addons.Option{}
		if c.Bool("no-prompt") {
			opts = append(opts, addons.WithoutPrompt())
		}
		for _, addon := range c.StringSlice("disable-addon") {
			opts = append(opts, addons.WithoutAddon(addon))
		}
		if err := addons.NewApplier(opts...).Apply(c.Context); err != nil {
			err := fmt.Errorf("unable to apply addons: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		if err := runPostApply(c.Context); err != nil {
			err := fmt.Errorf("unable to run post apply: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		fmt.Println("Cluster configuration has been applied")
		fmt.Printf("Kubeconfig file has been placed at at %s\n", kcfg)
		fmt.Printf("Cluster configuration file has been placed at %s\n", ccfg)
		fmt.Println("You can now access your cluster with kubectl by running:")
		fmt.Printf("  %s shell\n", os.Args[0])
		metrics.ReportApplyFinished(c, nil)
		return nil
	},
}
