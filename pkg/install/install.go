/*
Package install provides the functionality to install helmbin as a service on the host.
*/
package install

import (
	"fmt"

	"github.com/kardianos/service"
	"github.com/replicatedhq/helmbin/pkg/constants"
	"github.com/sirupsen/logrus"
)

const (
	svcDescription = "Embedded Kubernetes"
)

type program struct{}

func (p *program) Start(service.Service) error {
	// Start should not block. Do the actual work async.
	return nil
}

func (p *program) Stop(service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

// GetServiceConfig returns the service configuration for the given role
func GetServiceConfig(name, role string) *service.Config {
	var displayName, svcName string

	if role == constants.RoleController || role == constants.RoleWorker {
		displayName = name + " " + role
		svcName = name + role
	} else {
		displayName = name
		svcName = name
	}
	return &service.Config{
		Name:        svcName,
		DisplayName: displayName,
		Description: svcDescription,
	}
}

// InstalledService returns the service if one has been installed on the host or an error otherwise.
func InstalledService(name string) (service.Service, error) {
	prg := &program{}
	for _, role := range []string{constants.RoleController, constants.RoleWorker} {
		c := GetServiceConfig(name, role)
		s, err := service.New(prg, c)
		if err != nil {
			return nil, err
		}
		_, err = s.Status()

		if err != nil && err == service.ErrNotInstalled {
			continue
		}
		if err != nil {
			return nil, err
		}
		return s, nil
	}

	var s service.Service
	return s, fmt.Errorf("helmbin has not been installed as a service")
}

// EnsureService installs the service, per the given arguments, and the detected platform
func EnsureService(name string, args []string, envVars []string, force bool) error {
	var deps []string
	var svcConfig *service.Config

	prg := &program{}
	for _, v := range args {
		if v == constants.RoleController || v == constants.RoleWorker {
			svcConfig = GetServiceConfig(name, v)
			break
		}
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	// fetch service type
	svcType := s.Platform()
	switch svcType {
	// TODO: support for other platforms?
	case "linux-systemd":
		deps = []string{"After=network-online.target", "Wants=network-online.target"}
		svcConfig.Option = map[string]interface{}{
			"SystemdScript": systemdScript,
			"LimitNOFILE":   999999,
		}
	default:
	}

	if len(envVars) > 0 {
		svcConfig.Option["Environment"] = envVars
	}

	svcConfig.Dependencies = deps
	svcConfig.Arguments = args
	if force {
		logrus.Infof("Uninstalling %s service", svcConfig.Name)
		err = s.Uninstall()
		if err != nil && err != service.ErrNotInstalled {
			logrus.Warnf("failed to uninstall service: %v", err)
		}
	}
	logrus.Infof("Installing %s service", svcConfig.Name)
	err = s.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %v", err)
	}
	return nil
}

// UninstallService uninstalls the service for the given role
func UninstallService(name string, role string) error {
	prg := &program{}

	if role == constants.RoleControllerWorker {
		role = constants.RoleController
	}

	svcConfig := GetServiceConfig(name, role)
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	return s.Uninstall()
}

// Upstream kardianos/service does not support all the options we want to set to the systemd unit, hence we override the
// template.
// Currently mostly for KillMode=process so we get systemd to only send the sigterm to the main process
const systemdScript = `[Unit]
Description={{.Description}}
Documentation=https://docs.replicated.com
ConditionFileIsExecutable={{.Path|cmdEscape}}
{{range $i, $dep := .Dependencies}}
{{$dep}} {{end}}

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmdEscape}}{{end}}
{{- if .Option.Environment}}{{range .Option.Environment}}
Environment="{{.}}"{{end}}{{- end}}

RestartSec=120
Delegate=yes
KillMode=process
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0

{{- if .ChRoot}}RootDirectory={{.ChRoot|cmd}}{{- end}}

{{- if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory|cmdEscape}}{{- end}}
{{- if .UserName}}User={{.UserName}}{{end}}
{{- if .ReloadSignal}}ExecReload=/bin/kill -{{.ReloadSignal}} "$MAINPID"{{- end}}
{{- if .PIDFile}}PIDFile={{.PIDFile|cmd}}{{- end}}
{{- if and .LogOutput .HasOutputFileSupport -}}
StandardOutput=file:/var/log/{{.Name}}.out
StandardError=file:/var/log/{{.Name}}.err
{{- end}}

{{- if .SuccessExitStatus}}SuccessExitStatus={{.SuccessExitStatus}}{{- end}}
{{ if gt .LimitNOFILE -1 }}LimitNOFILE={{.LimitNOFILE}}{{- end}}
{{ if .Restart}}Restart={{.Restart}}{{- end}}

[Install]
WantedBy=multi-user.target
`
