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

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

const (
	azureSubscriptionIDKeyField = "azure-subscription-id"
	azureTenantIDKeyField       = "azure-tenant-id"
	azureClientIDKeyField       = "azure-client-id"
	azureClientSecretKeyField   = "azure-client-secret"
	azureResourceGroupParam     = "azure-resource-group"
	defaultAzureResourceGroup   = "docker-machine"
)

type AzureCredential struct {
	ClientID       string
	ClientSecret   string
	TenantID       string
	SubscriptionID string
}

func (r *MachineReconciler) deleteAzureResourceGroup() error {
	r.Log.Info("Deleting Azure Resource Group", "Name", r.machineObj.Name)
	azureCred, err := r.getAzureCredential()
	if err != nil {
		return err
	}
	cred, err := azidentity.NewClientSecretCredential(azureCred.TenantID, azureCred.ClientID, azureCred.ClientSecret, nil)
	if err != nil {
		return err
	}

	rgClient, err := armresources.NewResourceGroupsClient(azureCred.SubscriptionID, cred, nil)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	resourceGroupName := r.getResourceGroupName()
	poller, err := rgClient.BeginDelete(r.ctx, resourceGroupName, nil)
	if err != nil {
		return err
	}
	if _, err := poller.PollUntilDone(r.ctx, nil); err != nil {
		return err
	}

	return nil
}

func (r *MachineReconciler) getAzureCredential() (*AzureCredential, error) {
	authSecret, err := r.getSecret(r.machineObj.Spec.AuthSecret)
	if err != nil {
		return nil, err
	}

	if len(authSecret.Data[azureSubscriptionIDKeyField]) == 0 ||
		len(authSecret.Data[azureTenantIDKeyField]) == 0 ||
		len(authSecret.Data[azureClientIDKeyField]) == 0 ||
		len(authSecret.Data[azureClientSecretKeyField]) == 0 {
		return nil, fmt.Errorf("auth secret not found")
	}
	azureCred := AzureCredential{
		SubscriptionID: string(authSecret.Data[azureSubscriptionIDKeyField]),
		TenantID:       string(authSecret.Data[azureTenantIDKeyField]),
		ClientID:       string(authSecret.Data[azureClientIDKeyField]),
		ClientSecret:   string(authSecret.Data[azureClientSecretKeyField]),
	}
	return &azureCred, nil
}

func (r *MachineReconciler) getResourceGroupName() string {
	rgName, ok := r.machineObj.Spec.Parameters[azureResourceGroupParam]
	if !ok {
		r.Log.Info("Using default resource group docker-machine")
		rgName = defaultAzureResourceGroup
	}
	return rgName
}
