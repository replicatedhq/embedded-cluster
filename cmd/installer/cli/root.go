package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ErrorNothingElseToAdd is an error returned when there is nothing else to add to the screen. This
// is useful when we want to exit an error from a function here but don't want to print anything
// else (possibly because we have already printed the necessary data to the screen).
type ErrorNothingElseToAdd struct {
	Err error
}

func (e ErrorNothingElseToAdd) Error() string {
	return e.Err.Error()
}

func NewErrorNothingElseToAdd(err error) ErrorNothingElseToAdd {
	return ErrorNothingElseToAdd{
		Err: err,
	}
}

func InitAndExecute(ctx context.Context, name string) {
	cmd := RootCmd(ctx, name)
	err := cmd.Execute()
	if err != nil {
		if !errors.As(err, &ErrorNothingElseToAdd{}) {
			if isErrPermissionForkExec(err) {
				logrus.Errorf("\n" +
					"Execution is not permitted. Please ensure that the executable bit (x) is " +
					"set, the filesystem is not mounted with the 'noexec' option, and that " +
					"security policies such as SELinux or AppArmor are not blocking execution.\n")
			}
			// Logrus Fatal level logs to stderr and gets sent to the log file.
			logrus.Fatal("ERROR: ", err)
		}
		os.Exit(1)
	}
}

func RootCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           name,
		Short:         name,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dryrun.Enabled() {
				dryrun.RecordFlags(cmd.Flags())
			}

			// for any command that has an "airgap-bundle" flag, disable metrics
			if cmd.Flags().Lookup("airgap-bundle") != nil {
				v, err := cmd.Flags().GetString("airgap-bundle")
				if err != nil {
					return fmt.Errorf("unable to get airgap-bundle flag: %w", err)
				}

				if v != "" {
					metrics.DisableMetrics()
				}
			}

			if os.Getenv("DISABLE_TELEMETRY") != "" {
				metrics.DisableMetrics()
			}

			// set the umask to 022 so that we can create files/directories with 755 permissions
			// this does not return an error - it returns the previous umask
			_ = syscall.Umask(0o022)

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if dryrun.Enabled() {
				if err := dryrun.Dump(); err != nil {
					return fmt.Errorf("unable to dump dry run info: %w", err)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			os.Exit(1)
			return nil
		},
	}

	cmd.AddCommand(InstallCmd(ctx, name))
	cmd.AddCommand(JoinCmd(ctx, name))
	cmd.AddCommand(ShellCmd(ctx, name))
	cmd.AddCommand(NodeCmd(ctx, name))
	cmd.AddCommand(EnableHACmd(ctx, name))
	cmd.AddCommand(VersionCmd(ctx, name))
	cmd.AddCommand(ResetCmd(ctx, name))
	cmd.AddCommand(MaterializeCmd(ctx, name))
	cmd.AddCommand(UpdateCmd(ctx, name))
	cmd.AddCommand(RestoreCmd(ctx, name))
	cmd.AddCommand(AdminConsoleCmd(ctx, name))
	cmd.AddCommand(SupportBundleCmd(ctx, name))

	return cmd
}

func isErrPermissionForkExec(err error) bool {
	if !errors.Is(err, os.ErrPermission) {
		return false
	}
	var pathErr *os.PathError
	return errors.As(err, &pathErr) && pathErr.Op == "fork/exec"
}
