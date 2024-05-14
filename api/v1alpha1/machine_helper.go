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

import kmapi "kmodules.xyz/client-go/api/v1"

func (in *Machine) GetStatus() *MachineStatus {
	return &in.Status
}

func (in *Machine) GetConditions() kmapi.Conditions {
	return in.Status.Conditions
}

func (in *Machine) SetConditions(conditions kmapi.Conditions) {
	in.Status.Conditions = conditions
}
