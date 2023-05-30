package noderolenotaints

import (
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
)

type NodeRoleNoTaintsSuite struct {
	common.FootlooseSuite
}

func (s *NodeRoleNoTaintsSuite) TestK0sNoTaints() {
	s.NoError(s.InitController(0, "--enable-worker", "--no-taints"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeLabel(kc, s.ControllerNode(0), "node-role.kubernetes.io/control-plane", "true")
	s.NoError(err)

	n, err := kc.CoreV1().Nodes().Get(s.Context(), s.ControllerNode(0), v1.GetOptions{})
	s.NoError(err)
	s.NotContains(n.Spec.Taints, corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: "NoSchedule"})
}

func TestNodeRoleNoTaintsSuite(t *testing.T) {
	s := NodeRoleNoTaintsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
