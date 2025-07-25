package app

import (
	"testing"

	appinstall "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/stretchr/testify/suite"
)

func TestAppInstallControllerSuite(t *testing.T) {
	installTypes := []struct {
		name               string
		installType        string
		createStateMachine func(initialState statemachine.State) statemachine.Interface
	}{
		{
			name:        "linux install",
			installType: "linux",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(initialState))
			},
		},
		{
			name:        "kubernetes install",
			installType: "kubernetes",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(initialState))
			},
		},
	}

	for _, tt := range installTypes {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &appinstall.AppInstallControllerTestSuite{
				InstallType:        tt.installType,
				CreateStateMachine: tt.createStateMachine,
			}
			suite.Run(t, testSuite)
		})
	}
}
