/*
Copyright 2023.

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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

// MachineSpec defines the desired state of Machine
type MachineSpec struct {
	DriverRef  *core.LocalObjectReference `json:"driverRef"`
	ScriptRef  *core.LocalObjectReference `json:"scriptRef"`
	Parameters map[string]string          `json:"parameters"`
}

type MachinePhase string

const (
	MachinePhasePending     MachinePhase = "Pending"
	MachinePhaseInProgress  MachinePhase = "InProgress"
	MachinePhaseTerminating MachinePhase = "Terminating"
	MachinePhaseFailed      MachinePhase = "Failed"
)

// MachineStatus defines the observed state of Machine
type MachineStatus struct {
	Conditions []kmapi.Conditions `json:"conditions"`
	Phase      MachinePhase       `json:"phase"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Machine is the Schema for the machines API
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MachineList contains a list of Machine
type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Machine{}, &MachineList{})
}
