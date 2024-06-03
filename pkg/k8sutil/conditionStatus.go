package k8sutil

import (
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckConditionStatus(inStat v1beta1.InstallationStatus, conditionName string) v1.ConditionStatus {
	for _, cond := range inStat.Conditions {
		if cond.Type == conditionName {
			return cond.Status
		}
	}

	return ""
}
