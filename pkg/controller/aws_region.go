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

package controller

// This list generated using scripts/ami.sh file
var amiIDs = map[string]string{
	"ap-south-2":     "ami-00680cef23a721c2a",
	"ap-south-1":     "ami-0c2af51e265bd5e0e",
	"eu-south-1":     "ami-05dfcfdd4f0f2620f",
	"eu-south-2":     "ami-0549851bafe151f6c",
	"me-central-1":   "ami-0a389865294dadfe2",
	"il-central-1":   "ami-01bbc242f90fb2d97",
	"ca-central-1":   "ami-048ddca51ab3229ab",
	"eu-central-1":   "ami-07652eda1fbad7432",
	"eu-central-2":   "ami-02871ac2c044b004f",
	"us-west-1":      "ami-0ecaad63ed3668fca",
	"us-west-2":      "ami-0075013580f6322a1",
	"af-south-1":     "ami-0e806decb501416d7",
	"eu-north-1":     "ami-07a0715df72e58928",
	"eu-west-3":      "ami-0062b622072515714",
	"eu-west-2":      "ami-07d20571c32ba6cdc",
	"eu-west-1":      "ami-0932dacac40965a65",
	"ap-northeast-3": "ami-0a70c5266db4a6202",
	"ap-northeast-2": "ami-056a29f2eddc40520",
	"me-south-1":     "ami-0674e550ebeaf53d0",
	"ap-northeast-1": "ami-0162fe8bfebb6ea16",
	"sa-east-1":      "ami-01a38093d387a7497",
	"ap-east-1":      "ami-01412724cbc6252ef",
	"ca-west-1":      "ami-08fa6bf45a9f39b9e",
	"ap-southeast-1": "ami-0497a974f8d5dcef8",
	"ap-southeast-2": "ami-0375ab65ee943a2a6",
	"ap-southeast-3": "ami-06a2e6561950f5040",
	"ap-southeast-4": "ami-0d5de3084c18e52d9",
	"us-east-1":      "ami-0a0e5d9c7acc336f1",
	"us-east-2":      "ami-003932de22c285676",
}

func (r *MachineReconciler) getAMIIDArg() []string {
	args := []string{}
	region := r.machineObj.Spec.Parameters[awsRegionField]
	if amiIDs[region] != "" {
		args = append(args, "--amazonec2-ami", amiIDs[region])
	}

	return args
}
