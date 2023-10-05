package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/metrics"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

var tokenCommands = &cli.Command{
	Name:        "token",
	Usage:       "Manage node join tokens",
	Subcommands: []*cli.Command{tokenCreateCommand},
}

// JoinToken is a struct that holds both the actual token and the cluster id. This is marshaled
// and base64 encoded and used as argument to the join command in the other nodes.
type JoinToken struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Token     string    `json:"token"`
	Role      string    `json:"role"`
}

// Decode decodes a base64 encoded JoinToken.
func (j *JoinToken) Decode(b64 string) error {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, j)
}

// Encode encodes a JoinToken to base64.
func (j *JoinToken) Encode() (string, error) {
	b, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

var tokenCreateCommand = &cli.Command{
	Name:  "create",
	Usage: "Creates a new node join token",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "role",
			Usage: "The role of the token (can be controller or worker)",
			Value: "worker",
		},
		&cli.DurationFlag{
			Name:  "expiry",
			Usage: "For how long the token should be valid",
			Value: 24 * time.Hour,
		},
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Do not prompt user when it is not necessary",
			Value: false,
		},
	},
	Action: func(c *cli.Context) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("token create is only supported on linux")
		}
		if os.Getuid() != 0 {
			return fmt.Errorf("token create must be run as root")
		}
		role := c.String("role")
		if role != "worker" && role != "controller" {
			return fmt.Errorf("invalid role %q", role)
		}
		useprompt := !c.Bool("no-prompt")
		cfgpath := defaults.PathToConfig("k0sctl.yaml")
		if _, err := os.Stat(cfgpath); err != nil {
			if os.IsNotExist(err) {
				logrus.Errorf("Unable to find the cluster configuration.")
				logrus.Errorf("Consider running the command using 'sudo -E'")
				return fmt.Errorf("configuration not found")
			}
			return fmt.Errorf("unable to stat k0sctl.yaml: %w", err)
		}
		logrus.Infof("Creating node join token for role %s", role)
		if !defaults.DecentralizedInstall() {
			fmt.Println("You are opting out of the centralized cluster management.")
			fmt.Println("Through the centralized management you can manage all your")
			fmt.Println("cluster nodes from a single location. If you decide to move")
			fmt.Println("on the centralized management won't be available anymore")
			if useprompt && !prompts.New().Confirm("Do you want to continue ?", true) {
				return nil
			}
		}
		dur := c.Duration("expiry").String()
		buf := bytes.NewBuffer(nil)
		cmd := exec.Command("k0s", "token", "create", "--expiry", dur, "--role", role)
		cmd.Stdout = buf
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}
		if !defaults.DecentralizedInstall() {
			if err := defaults.SetInstallAsDecentralized(); err != nil {
				return fmt.Errorf("unable to set decentralized install: %w", err)
			}
		}
		token := JoinToken{metrics.ClusterID(), buf.String(), role}
		b64token, err := token.Encode()
		if err != nil {
			return fmt.Errorf("unable to encode token: %w", err)
		}
		fmt.Println("Token created successfully.")
		fmt.Printf("This token is valid for %s hours.\n", dur)
		fmt.Println("You can now run the following command in a remote node to add it")
		fmt.Printf("to the cluster as a %q node:\n", role)
		fmt.Printf("%s node join %s\n", defaults.BinaryName(), b64token)
		return nil
	},
}
