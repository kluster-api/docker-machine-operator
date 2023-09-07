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
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/klog/v2"
	"time"
)

const (
	awsAccessKeyField              = "amazonec2-access-key"
	awsSecretKeyField              = "amazonec2-secret-key"
	awsRegionField                 = "amazonec2-region"
	awsVPCIDAnnotation             = "docker-machine-operator/aws-vpc"
	awsSubnetIDAnnotation          = "docker-machine-operator/aws-subnet"
	awsInternetGatewayIDAnnotation = "docker-machine-operator/aws-gateway"
	awsVpcCIDR                     = "10.1.0.0/16"
	allowAllIPs                    = "0.0.0.0/0"
)

type awsAuthCredential struct {
	accessKey, secretKey, region string
}

func (r *MachineReconciler) getAWSCredentials() (*awsAuthCredential, error) {
	authSecret, err := r.getSecret(r.machineObj.Spec.AuthSecret)
	if err != nil {
		return nil, err
	}

	var awsCreds awsAuthCredential
	for key, value := range authSecret.Data {
		data := string(value)
		if len(data) == 0 || len(key) == 0 {
			return nil, fmt.Errorf("auth secret not found")
		}
		if key == awsAccessKeyField {
			awsCreds.accessKey = data
		}
		if key == awsSecretKeyField {
			awsCreds.secretKey = data
		}
	}
	awsCreds.region = r.machineObj.Spec.Parameters[awsRegionField]
	if awsCreds.secretKey == "" || awsCreds.accessKey == "" || awsCreds.region == "" {
		return nil, errors.New("failed to get aws credentials or region")
	}
	return &awsCreds, nil
}

func (r *MachineReconciler) newAWSClientSession() (*session.Session, error) {
	cred, err := r.getAWSCredentials()
	if err != nil {
		return nil, err
	}

	awsCfg := &aws.Config{
		Region:      &cred.region,
		Credentials: credentials.NewStaticCredentials(cred.accessKey, cred.secretKey, ""),
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (r *MachineReconciler) awsEC2Client() (*ec2.EC2, error) {
	sess, err := r.newAWSClientSession()
	if err != nil {
		return nil, err
	}
	return ec2.New(sess), nil
}

func (r *MachineReconciler) createAwsRoute(c *ec2.EC2, vpcId, internetGatewayID string) error {
	out, err := c.DescribeRouteTables(&ec2.DescribeRouteTablesInput{})
	if err != nil {
		return err
	}
	var routeTable *ec2.RouteTable
	for _, rt := range out.RouteTables {
		if *rt.VpcId == vpcId {
			routeTable = rt
		}
	}
	if routeTable == nil {
		return fmt.Errorf("no route table found in vpc %s", vpcId)
	}

	_, err = c.CreateRoute(&ec2.CreateRouteInput{
		DestinationCidrBlock: stringToP(allowAllIPs),
		GatewayId:            &internetGatewayID,
		RouteTableId:         routeTable.RouteTableId,
	})
	if err != nil {
		return err
	}

	klog.Infof("route table updated")
	return nil
}

func attachInternetGatewayToVPC(c *ec2.EC2, internetGatewayId, vpcID string) error {
	_, err := c.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		InternetGatewayId: &internetGatewayId,
		VpcId:             &vpcID,
	})
	return err
}
func (r *MachineReconciler) createAwsInternetGateway(c *ec2.EC2, vpcId string) error {
	out, err := c.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	if err != nil {
		return err
	}
	err = attachInternetGatewayToVPC(c, *out.InternetGateway.InternetGatewayId, vpcId)
	if err != nil {
		er := r.deleteAwsInternetGateway(c, *out.InternetGateway.InternetGatewayId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	err = r.createAwsRoute(c, vpcId, *out.InternetGateway.InternetGatewayId)
	if err != nil {
		er := r.deleteAwsInternetGateway(c, *out.InternetGateway.InternetGatewayId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	klog.Infof("internet gateway is created with id: %s", *out.InternetGateway.InternetGatewayId)
	r.machineObj.Annotations[awsInternetGatewayIDAnnotation] = *out.InternetGateway.InternetGatewayId
	return nil
}
func (r *MachineReconciler) deleteAwsInternetGateway(c *ec2.EC2, gatewayId string) error {
	_, err := c.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
		DryRun:            nil,
		InternetGatewayId: &gatewayId,
	})
	if err != nil {
		return err
	}
	r.machineObj.Annotations[awsInternetGatewayIDAnnotation] = ""
	klog.Infof("internet gateway successfully deleted")
	return nil
}

func (r *MachineReconciler) createAwsSubnet(c *ec2.EC2, vpcID string) error {
	out, err := c.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock: stringToP(awsVpcCIDR),
		VpcId:     &vpcID,
	})
	if err != nil {
		return err
	}

	err = r.createAwsInternetGateway(c, vpcID)
	if err != nil {
		er := r.deleteAwsSubnet(c, *out.Subnet.SubnetId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	r.machineObj.Annotations[awsSubnetIDAnnotation] = *out.Subnet.SubnetId
	klog.Infof("aws subnet created with subnet id: %s", *out.Subnet.SubnetId)
	return nil
}
func (r *MachineReconciler) deleteAwsSubnet(c *ec2.EC2, subnetId string) error {
	if r.machineObj.Annotations[awsInternetGatewayIDAnnotation] != "" {
		_ = r.deleteAwsInternetGateway(c, r.machineObj.Annotations[awsInternetGatewayIDAnnotation])
	}
	_, err := c.DeleteSubnet(&ec2.DeleteSubnetInput{
		SubnetId: &subnetId,
	})
	if err != nil {
		return err
	}

	r.machineObj.Annotations[awsSubnetIDAnnotation] = ""
	klog.Infof("subnet successfully deleted")
	return nil
}

func getVPC(c *ec2.EC2, vpcId *string) (*ec2.Vpc, error) {
	out, err := c.DescribeVpcs(&ec2.DescribeVpcsInput{
		VpcIds: stringPSlice([]string{*vpcId}),
	})
	if err != nil {
		return nil, err
	}

	for _, vpc := range out.Vpcs {
		if *vpc.VpcId == *vpcId {
			return vpc, nil
		}
	}
	return nil, fmt.Errorf("no vpc found with id %s", *vpcId)
}
func (r *MachineReconciler) createAwsVpc(c *ec2.EC2) error {
	out, err := c.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: stringToP(awsVpcCIDR),
	})
	if err != nil {
		return err
	}

	err = waitForState(5*time.Second, 1*time.Minute, func() (bool, error) {
		vpc, err := getVPC(c, out.Vpc.VpcId)
		if err != nil {
			return false, err
		}

		if *vpc.State == "available" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		er := r.deleteAwsVpc(c, *out.Vpc.VpcId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	err = r.createAwsSubnet(c, *out.Vpc.VpcId)
	if err != nil {
		er := r.deleteAwsVpc(c, *out.Vpc.VpcId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	r.machineObj.Annotations[awsVPCIDAnnotation] = *out.Vpc.VpcId
	klog.Infof("aws vpc created with id %s", *out.Vpc.VpcId)
	return nil
}
func (r *MachineReconciler) deleteAwsVpc(c *ec2.EC2, vpcID string) error {
	if r.machineObj.Annotations[awsSubnetIDAnnotation] != "" {
		_ = r.deleteAwsSubnet(c, r.machineObj.Annotations[awsSubnetIDAnnotation])
	}
	_, err := c.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: stringToP(vpcID),
	})
	if err != nil {
		return err
	}

	r.machineObj.Annotations[awsVPCIDAnnotation] = ""
	klog.Infof("vpc successfully delete")
	return nil
}

func (r *MachineReconciler) createAWSEnvironment() error {
	if r.machineObj.Annotations[awsVPCIDAnnotation] != "" && r.machineObj.Annotations[awsSubnetIDAnnotation] != "" && r.machineObj.Annotations[awsInternetGatewayIDAnnotation] != "" {
		return nil
	}

	c, err := r.awsEC2Client()
	if err != nil {
		return err
	}
	return r.createAwsVpc(c)
}
