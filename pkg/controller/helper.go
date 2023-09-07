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

import (
	"fmt"
	"time"
)

func stringToP(st string) *string {
	return &st
}
func stringPSlice(sl []string) []*string {
	var ret []*string
	for i := 0; i < len(sl); i++ {
		ret = append(ret, &sl[i])
	}
	return ret
}

func waitForState(retry, timeout time.Duration, getStatus func() (bool, error)) error {
	for t := time.Second * 0; t <= timeout; t += retry {
		fmt.Println("getting state")
		res, err := getStatus()
		if err != nil {
			return err
		}
		if res {
			return nil
		}
		fmt.Println("retrying")
		time.Sleep(retry)
	}
	return fmt.Errorf("failed to get desired status")
}
