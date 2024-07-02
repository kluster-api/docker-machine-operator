#!/bin/bash

# Copyright AppsCode Inc. and Contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.



regions=$(aws ec2 describe-regions --query "Regions[].RegionName" --output json | jq -r '.[]')

for region in $regions; do
    ami_id=$(aws ec2 describe-images \
        --owners "099720109477" \
        --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*" \
        --query "Images | sort_by(@, &CreationDate) | [-1].[ImageId]" \
        --output text \
        --region "$region")
    echo "\"$region\": \"$ami_id\","
done
