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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/klog/v2"
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
	defaultZone                    = "a" //same as rancher amazonec2 driver default zone
	regionParameter                = "amazonec2-region"
)

type awsAuthCredential struct {
	accessKey, secretKey, region string
}

func (r *MachineReconciler) getAWSCredentials() (*awsAuthCredential, error) {
	authSecret, err := r.getSecret(r.machineObj.Spec.AuthSecret)
	if err != nil {
		return nil, err
	}

	if len(authSecret.Data[awsAccessKeyField]) == 0 || len(authSecret.Data[awsSecretKeyField]) == 0 {
		return nil, fmt.Errorf("auth secret not found")
	}
	awsCreds := awsAuthCredential{
		accessKey: string(authSecret.Data[awsAccessKeyField]),
		secretKey: string(authSecret.Data[awsSecretKeyField]),
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

	session, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *MachineReconciler) awsEC2Client() (*ec2.EC2, error) {
	sess, err := r.newAWSClientSession()
	if err != nil {
		return nil, err
	}
	return ec2.New(sess), nil
}

func createAwsRoute(c *ec2.EC2, vpcId, internetGatewayID string) error {
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
func deleteAwsRoute(c *ec2.EC2, vpcId string) error {
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

	_, err = c.DeleteRoute(&ec2.DeleteRouteInput{
		DestinationCidrBlock: stringToP(allowAllIPs),
		RouteTableId:         routeTable.RouteTableId,
	})
	if err != nil {
		return err
	}
	klog.Infof("route deleted")
	return nil
}

func attachInternetGatewayToVPC(c *ec2.EC2, internetGatewayId, vpcID string) error {
	_, err := c.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		InternetGatewayId: &internetGatewayId,
		VpcId:             &vpcID,
	})
	return err
}
func detachInternetGatewayToVPC(c *ec2.EC2, internetGatewayID, vpcID string) error {
	_, err := c.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
		InternetGatewayId: &internetGatewayID,
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
		er := deleteAwsInternetGateway(c, *out.InternetGateway.InternetGatewayId, vpcId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	err = createAwsRoute(c, vpcId, *out.InternetGateway.InternetGatewayId)
	if err != nil {
		er := deleteAwsInternetGateway(c, *out.InternetGateway.InternetGatewayId, vpcId)
		if er != nil {
			err = errors.Join(err, er)
		}
		return err
	}

	if err = r.patchAnnotation(awsInternetGatewayIDAnnotation, *out.InternetGateway.InternetGatewayId); err != nil {
		return err
	}

	klog.Infof("internet gateway is created with id: %s", *out.InternetGateway.InternetGatewayId)
	return nil
}
func deleteAwsInternetGateway(c *ec2.EC2, gatewayId, vpcId string) error {
	if err := deleteAwsRoute(c, vpcId); err != nil {
		klog.Warningf("failed to delete route, ", err.Error())
	}
	if err := detachInternetGatewayToVPC(c, gatewayId, vpcId); err != nil {
		klog.Warningf("failed to detach internet gateway to VPC, ", err.Error())
	}
	_, err := c.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
		DryRun:            nil,
		InternetGatewayId: &gatewayId,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			klog.Warningf(err.Error())
			return nil
		}
		return err
	}

	klog.Infof("internet gateway successfully deleted")
	return nil
}

func (r *MachineReconciler) createAwsSubnet(c *ec2.EC2, vpcID string) error {
	if r.machineObj.Spec.Parameters[regionParameter] == "" {
		return errors.New("region not specified")
	}
	out, err := c.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock:        stringToP(awsVpcCIDR),
		VpcId:            &vpcID,
		AvailabilityZone: stringToP(fmt.Sprintf("%s%s", r.machineObj.Spec.Parameters[regionParameter], defaultZone)),
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

	if err = r.patchAnnotation(awsSubnetIDAnnotation, *out.Subnet.SubnetId); err != nil {
		return err
	}

	klog.Infof("aws subnet created with subnet id: %s", *out.Subnet.SubnetId)
	return nil
}
func (r *MachineReconciler) deleteAwsSubnet(c *ec2.EC2, subnetId string) error {
	if r.machineObj.Annotations[awsInternetGatewayIDAnnotation] != "" {
		if err := deleteAwsInternetGateway(c, r.machineObj.Annotations[awsInternetGatewayIDAnnotation], r.machineObj.Annotations[awsVPCIDAnnotation]); err != nil {
			klog.Warningf("failed to delete internet gateway, ", err.Error())
		}
	}
	_, err := c.DeleteSubnet(&ec2.DeleteSubnetInput{
		SubnetId: &subnetId,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			klog.Warningf(err.Error())
			return nil
		}
		return err
	}

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
	var vpc *ec2.Vpc
	var err error

	if r.machineObj.Annotations[awsVPCIDAnnotation] == "" {
		out, err := c.CreateVpc(&ec2.CreateVpcInput{
			CidrBlock: stringToP(awsVpcCIDR),
		})
		if err != nil {
			return err
		}
		vpc = out.Vpc
		if err = r.patchAnnotation(awsVPCIDAnnotation, *vpc.VpcId); err != nil {
			return err
		}

		err = waitForState(5*time.Second, 1*time.Minute, func() (bool, error) {
			vpc, err := getVPC(c, vpc.VpcId)
			if err != nil {
				return false, err
			}

			if *vpc.State == "available" {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			er := r.deleteAwsVpc(c, *vpc.VpcId)
			if er != nil {
				err = errors.Join(err, er)
			}
			return err
		}
	} else {
		vpc, err = getVPC(c, stringToP(r.machineObj.Annotations[awsVPCIDAnnotation]))
		if err != nil {
			return err
		}
	}

	if r.machineObj.Annotations[awsSubnetIDAnnotation] == "" {
		if err = r.createAwsSubnet(c, *vpc.VpcId); err != nil {
			er := r.deleteAwsVpc(c, *vpc.VpcId)
			if er != nil {
				err = errors.Join(err, er)
			}
			return err
		}
	}

	klog.Infof("aws vpc created with id %s", *vpc.VpcId)
	return nil
}
func (r *MachineReconciler) deleteAwsVpc(c *ec2.EC2, vpcID string) error {
	if r.machineObj.Annotations[awsVPCIDAnnotation] == "" {
		return nil
	}

	if r.machineObj.Annotations[awsSubnetIDAnnotation] != "" {
		_ = r.deleteAwsSubnet(c, r.machineObj.Annotations[awsSubnetIDAnnotation])
	}

	if err := deleteSecurityGroup(c, vpcID); err != nil {
		return err
	}

	_, err := c.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: stringToP(vpcID),
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			klog.Warningf(err.Error())
			return nil
		}
		return err
	}

	klog.Infof("vpc successfully delete")
	return nil
}

func deleteSecurityGroup(c *ec2.EC2, vpcId string) error {
	vpcKey := "vpc-id"
	filters := []*ec2.Filter{
		{
			Name:   &vpcKey,
			Values: stringPSlice([]string{vpcId}),
		},
	}
	des, err := c.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})
	if err != nil {
		return err
	}
	for _, sg := range des.SecurityGroups {
		_, err = c.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		})
		if err != nil {
			klog.Warningf(fmt.Sprintf("failed to delete security group: %s", *sg.GroupId))
		}
		klog.Infof("%s deleted", *sg.GroupId)
	}
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
