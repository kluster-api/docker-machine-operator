package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

type MachinePhase string

const (
	MachineConditionTypeMachineReady  kmapi.ConditionType = "MachineReady"
	MachineConditionTypeScriptReady   kmapi.ConditionType = "ScriptReady"
	MachineConditionTypeAuthDataReady kmapi.ConditionType = "AuthDataReady"
)

const (
	MachineConditionAuthDataNotFound   = "AuthDataNotFound"
	MachineConditionScriptDataNotFound = "ScriptDataNotFound"

	MachineConditionMachineCreating = "MachineCreating"
)

const (
	MachinePhasePending     MachinePhase = "Pending"
	MachinePhaseInProgress  MachinePhase = "InProgress"
	MachinePhaseSuccess     MachinePhase = "Success"
	MachinePhaseTerminating MachinePhase = "Terminating"
	MachinePhaseFailed      MachinePhase = "Failed"
)

func ConditionsOrder() []kmapi.ConditionType {
	return []kmapi.ConditionType{
		MachineConditionTypeMachineReady,
		MachineConditionTypeAuthDataReady,
		MachineConditionTypeScriptReady,
	}
}

func GetPhase(obj *Machine) MachinePhase {
	if !obj.GetDeletionTimestamp().IsZero() {
		return MachinePhaseTerminating
	}
	conditions := obj.GetConditions()
	var cond kmapi.Condition
	for i, _ := range conditions {
		c := conditions[i]
		if c.Type == kmapi.ReadyCondition {
			cond = c
			break
		}
	}
	if cond.Type != kmapi.ReadyCondition {
		panic(fmt.Sprintf("no Ready condition in the status for %s/%s", obj.GetNamespace(), obj.GetName()))
	}

	if cond.Status == metav1.ConditionTrue {
		return MachinePhaseSuccess
	}

	if cond.Reason == MachineConditionAuthDataNotFound ||
		cond.Reason == MachineConditionScriptDataNotFound {
		return MachinePhaseFailed
	}
	if cond.Reason == MachineConditionMachineCreating {
		return MachinePhaseInProgress
	}
	return MachinePhaseSuccess
}