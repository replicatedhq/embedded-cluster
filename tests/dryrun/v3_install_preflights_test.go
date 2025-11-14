package dryrun

import (
	"errors"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestV3InstallHeadless_HostPreflights_WithFailuresBlocking(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "host preflight checks completed with failures")

	// PostRun hooks do not run if the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	dr, err := dryrun.Load()
	require.NoError(t, err, "fail to load dryrun output")

	preflightRunner.AssertExpectations(t)

	// Validate metrics events
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `\"message\":\"Test check failed\"`) // preflight output
				assert.Contains(t, payload, `"eventType":"PreflightsFailed"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationFailed"`)
			},
		},
	})

	if !t.Failed() {
		t.Logf("Test passed: host preflight failures blocking installation")
	}
}

func TestV3InstallHeadless_HostPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command with ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-host-preflights",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// PostRun hooks do not run if the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	dr, err := dryrun.Load()
	require.NoError(t, err, "fail to load dryrun output")

	validateHostPreflightsWithFailuresBypass(t, dr, preflightRunner)

	if !t.Failed() {
		t.Logf("Test passed: --ignore-host-preflights flag correctly bypasses host preflight failures")
	}
}

func TestV3Install_HostPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--ignore-host-preflights",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: true, // Bypass host preflight failures
		ignoreAppPreflights:  false,
	})

	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	dr, err := dryrun.Load()
	require.NoError(t, err, "fail to load dryrun output")

	validateHostPreflightsWithFailuresBypass(t, dr, preflightRunner)

	if !t.Failed() {
		t.Logf("Test passed: ignoreHostPreflights flag correctly bypasses host preflight failures via API")
	}
}

func validateHostPreflightsWithFailuresBypass(t *testing.T, dr *types.DryRun, preflightRunner *preflights.MockPreflightRunner) {
	preflightRunner.AssertExpectations(t)

	// Validate metrics events
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `\"message\":\"Test check failed\"`) // preflight output
				assert.Contains(t, payload, `"eventType":"PreflightsFailed"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `\"message\":\"Test check failed\"`) // preflight output
				assert.Contains(t, payload, `"eventType":"PreflightsBypassed"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"AppPreflightsSucceeded"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationSucceeded"`)
			},
		},
	})
}

func TestV3InstallHeadless_HostPreflights_Fail(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: preflights failed to run")
	}
}

func TestV3InstallHeadless_HostPreflights_FailNoBypass(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-host-preflights",
		"--yes",
	)

	require.ErrorContains(t, err, "preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: preflights failed to run")
	}
}

func TestV3InstallHeadless_AppPreflights_WithFailuresBlocking(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command without ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflight checks completed with failures")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflight failures blocking installation")
	}
}

// TODO: TestV3Install_AppPreflights_WithFailuresBlocking

func TestV3InstallHeadless_AppPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command with ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-app-preflights",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: --ignore-app-preflights flag correctly bypasses app preflight failures")
	}
}

func TestV3Install_AppPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--ignore-app-preflights",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  true, // Bypass app preflight failures
	})

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: ignoreAppPreflights flag correctly bypasses app preflight failures via API")
	}
}

func TestV3InstallHeadless_AppPreflights_Fail(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command without ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflights execution failed")
	}
}

func TestV3InstallHeadless_AppPreflights_FailNoBypass(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		preflightRunner: preflightRunner,
	})

	// Run installer command with ignore-app-preflights flag - execution failures cannot be bypassed
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-app-preflights",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflights execution failure cannot be bypassed")
	}
}

func setupV3TestPreflightsRunner() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestHostPreflightsRunnerWithFail() *preflights.MockPreflightRunner {
	opts := preflights.RunOptions{
		PreflightBinaryPath: "/var/lib/fake-app-slug/bin/kubectl-preflight",
		ProxySpec:           nil,
		ExtraPaths:          []string{"/var/lib/fake-app-slug/bin"},
	}

	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.AnythingOfType("*v1beta2.HostPreflightSpec"), opts).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
			Fail: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check failed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Maybe(). // Will run if bypass flag is set
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestHostPreflightsRunnerFail() *preflights.MockPreflightRunner {
	opts := preflights.RunOptions{
		PreflightBinaryPath: "/var/lib/fake-app-slug/bin/kubectl-preflight",
		ProxySpec:           nil,
		ExtraPaths:          []string{"/var/lib/fake-app-slug/bin"},
	}

	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.AnythingOfType("*v1beta2.HostPreflightSpec"), opts).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(nil, "exit code 1", errors.New("preflights failed to run"))
	return preflightRunner
}

func setupV3TestAppPreflightsRunnerWithFail() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
			Fail: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check failed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestAppPreflightsRunnerFail() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(nil, "exit code 1", errors.New("app preflights failed to run"))
	return preflightRunner
}
