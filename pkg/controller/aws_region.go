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

// This list generated using scripts/ami.sh file and
// https://cloud-images.ubuntu.com/locator/ec2/
var amiIDs = map[string]string{
	"ap-south-1":     "ami-0fc76d1fc60a822e2",
	"eu-north-1":     "ami-09619f1544394c186",
	"eu-west-3":      "ami-0e7bc13af71a7ace1",
	"eu-west-2":      "ami-0ef73863dbddaa97f",
	"eu-west-1":      "ami-07e2abe41a3dd4483",
	"ap-northeast-3": "ami-06c6d2fb039aedeb2",
	"ap-northeast-2": "ami-0e73d7a01dba794a4",
	"ap-northeast-1": "ami-07ac8e5b1fefaa9e5",
	"ca-central-1":   "ami-0cd354c076a038948",
	"sa-east-1":      "ami-07f5d4171b892af81",
	"ap-southeast-1": "ami-09329fa58ea564a19",
	"ap-southeast-2": "ami-0266d373beca3c7fa",
	"eu-central-1":   "ami-0b6226e8488760b25",
	"us-east-1":      "ami-0111190769c4329ae",
	"us-east-2":      "ami-0b00f5183519c196c",
	"us-west-1":      "ami-0b1da0df74f3b4b0b",
	"us-west-2":      "ami-03ab9db8dada95e36",
	"me-central-1":   "ami-0170c48a53a986edc",
	"eu-south-1":     "ami-0494d18a3b83a9b91",
	"il-central-1":   "ami-079087d4fb0b82225",
	"af-south-1":     "ami-01015442638eed255",
	"cn-northwest-1": "ami-00978db6a05d06273",
	"cn-north-1":     "ami-0b820a65426833e1f",
	"ap-east-1":      "ami-06aacaba45deda8f5",
	"ca-west-1":      "ami-0989e7880da01e922",
	"me-south-1":     "ami-004a63c72a7208abc",
	"eu-south-2":     "ami-0de5f498466655517",
	"eu-central-2":   "ami-0868b61eaac7c1688",
	"ap-south-2":     "ami-09fd1f9e60e374015",
	"ap-southeast-3": "ami-002fde9f4f8bf8faf",
	"ap-southeast-4": "ami-010a2c002f5940231",
}

func (r *MachineReconciler) getAMIIDArg() []string {
	args := []string{}
	region := r.machineObj.Spec.Parameters[awsRegionField]
	if amiIDs[region] != "" {
		args = append(args, "--amazonec2-ami", amiIDs[region])
	}

	return args
}
