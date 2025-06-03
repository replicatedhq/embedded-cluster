package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ory/dockertest/v3"
	"github.com/replicatedhq/embedded-cluster/tests/integration/k0s/run/vm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	err := RootCmd().Execute()
	cobra.CheckErr(err)
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run TEST",
		Short: "Run tests for k0s in a VM",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			viper.BindPFlags(cmd.Flags())
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			tests, err := listTests(args[0])
			if err != nil {
				return fmt.Errorf("failed to list tests: %s", err)
			}

			for _, test := range tests {
				err := runTest(test)
				if err != nil {
					return fmt.Errorf("%s: %s", test, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().String("test-binary", "./k0s.test", "Path to the k0s test binary")
	cmd.Flags().String("data-dir", "/var/lib/embedded-cluster", "Path to the data directory")
	cmd.Flags().String("k0s-binary", "bin/k0s", "Path to the k0s binary")

	cobra.OnInitialize(func() {
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.SetEnvPrefix("TEST")
		viper.AutomaticEnv()
	})

	return cmd
}

func listTests(pattern string) ([]string, error) {
	// TODO: change to use pattern rather than test name
	return []string{pattern}, nil
}

func runTest(testName string) error {
	vmName := generateName(testName)

	k0sDataDir, err := os.MkdirTemp("", "k0s-data-dir-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %s", err)
	}
	defer func() {
		if os.Getenv("DEBUG") != "" {
			log.Printf("skipping k0s data dir remove for %s", vmName)
			return
		}
		os.RemoveAll(k0sDataDir)
	}()

	k0sBinary := viper.GetString("k0s-binary")
	k0sBinary, err = filepath.Abs(k0sBinary)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %s", err)
	}
	_, err = os.Stat(k0sBinary)
	if err != nil {
		return fmt.Errorf("k0s binary not found: %s", err)
	}

	testBinary := viper.GetString("test-binary")
	testBinary, err = filepath.Abs(testBinary)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %s", err)
	}
	_, err = os.Stat(testBinary)
	if err != nil {
		return fmt.Errorf("test binary not found: %s", err)
	}

	vm, err := vm.Create(vmName, k0sBinary, testBinary, nil)
	if err != nil {
		return fmt.Errorf("failed to create vm: %s", err)
	}
	defer func() {
		if os.Getenv("DEBUG") != "" {
			log.Printf("skipping vm close for %s", vmName)
			return
		}
		err := vm.Close()
		if err != nil {
			fmt.Printf("failed to close vm: %s", err)
		}
	}()

	stdin, stdout, stderr := os.Stdin, os.Stdout, os.Stderr
	opts := dockertest.ExecOptions{
		StdIn:  stdin,
		StdOut: stdout,
		StdErr: stderr,
	}
	exitCode, err := vm.Resource.Exec([]string{
		"k0s.test", "-test.v", "-test.timeout=5m", "-test.run", fmt.Sprintf("^%s$", testName),
	}, opts)
	if err != nil {
		return fmt.Errorf("failed to install k0s: %s", err)
	} else if exitCode != 0 {
		return fmt.Errorf("test failed with exit code %d", exitCode)
	}

	return nil
}

func generateName(testName string) string {
	return fmt.Sprintf("inttest-%s",
		strings.ReplaceAll(
			strings.ToLower(testName),
			"_", "-",
		),
	)
}
