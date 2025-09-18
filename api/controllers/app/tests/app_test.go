package tests

import (
	"testing"

	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	kubernetesupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/stretchr/testify/suite"
)

func TestAppControllerSuite(t *testing.T) {
	installTypes := []struct {
		name                      string
		installType               string
		createInstallStateMachine func(initialState statemachine.State) statemachine.Interface
		createUpgradeStateMachine func(initialState statemachine.State) statemachine.Interface
	}{
		{
			name:        "linux install",
			installType: "linux",
			createInstallStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(initialState))
			},
			createUpgradeStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(initialState))
			},
		},
		{
			name:        "kubernetes install",
			installType: "kubernetes",
			createInstallStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(initialState))
			},
			createUpgradeStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return kubernetesupgrade.NewStateMachine(kubernetesupgrade.WithCurrentState(initialState))
			},
		},
	}

	for _, tt := range installTypes {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &AppControllerTestSuite{
				InstallType:               tt.installType,
				CreateInstallStateMachine: tt.createInstallStateMachine,
				CreateUpgradeStateMachine: tt.createUpgradeStateMachine,
			}
			suite.Run(t, testSuite)
		})
	}
}
