/*
Copyright AppsCode Inc. and Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

type MachinePhase string

const (
	MachineConditionTypeMachineReady             kmapi.ConditionType = "MachineReady"
	MachineConditionTypeScriptReady              kmapi.ConditionType = "ScriptReady"
	MachineConditionTypeAuthDataReady            kmapi.ConditionType = "AuthDataReady"
	MachineConditionTypeClusterOperationComplete kmapi.ConditionType = "ClusterOperationComplete"
	MachineConditionTypeMachineCreating          kmapi.ConditionType = "MachineCreating"
)

const (
	ReasonClusterOperationFailed     = "ClusterOperationFailed"
	ReasonMachineCreationFailed      = "MachineCreationFailed"
	ReasonWaitingForScriptCompletion = "WaitingForScriptCompletion"
	ReasonWaitingForScriptRun        = "WaitingForScriptRun"
	ReasonAuthDataNotFound           = "AuthDataNotFound"
	ReasonScriptDataNotFound         = "ScriptDataNotFound"
	ReasonMachineCreating            = "MachineCreating"
)

const (
	MachinePhasePending                    MachinePhase = "Pending"
	MachinePhaseInProgress                 MachinePhase = "InProgress"
	MachinePhaseWaitingForScriptCompletion MachinePhase = "WaitingForScriptCompletion"
	MachinePhaseClusterOperationFailed     MachinePhase = "ClusterOperationFailed"
	MachinePhaseSuccess                    MachinePhase = "Success"
	MachinePhaseTerminating                MachinePhase = "Terminating"
	MachinePhaseFailed                     MachinePhase = "Failed"
)

func ConditionsOrder() []kmapi.ConditionType {
	return []kmapi.ConditionType{
		MachineConditionTypeMachineReady,
		MachineConditionTypeClusterOperationComplete,
		MachineConditionTypeAuthDataReady,
		MachineConditionTypeScriptReady,
	}
}

func GetPhase(obj *Machine) MachinePhase {
	if !obj.GetDeletionTimestamp().IsZero() {
		return MachinePhaseTerminating
	}
	conditions := obj.GetConditions()
	if len(conditions) == 0 {
		return MachinePhaseInProgress
	}
	var cond kmapi.Condition
	for i := range conditions {
		c := conditions[i]
		if c.Type == kmapi.ReadyCondition {
			cond = c
			break
		}
	}
	if cond.Type != kmapi.ReadyCondition {
		fmt.Printf("no Ready condition in the status for %s/%s", obj.GetNamespace(), obj.GetName())
		return MachinePhasePending
	}

	if cond.Status == metav1.ConditionTrue {
		return MachinePhaseSuccess
	}

	if cond.Reason == ReasonWaitingForScriptCompletion {
		return MachinePhaseWaitingForScriptCompletion
	}
	if cond.Reason == ReasonClusterOperationFailed {
		return MachinePhaseClusterOperationFailed
	}
	if cond.Reason == ReasonMachineCreationFailed {
		return MachinePhaseFailed
	}
	return MachinePhaseInProgress
}

func GetFinalizer() string {
	return GroupVersion.Group
}
