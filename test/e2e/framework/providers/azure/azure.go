//go:build !providerless
// +build !providerless

/*
Copyright 2018 The Kubernetes Authors.

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

package azure

import (
	"errors"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/legacy-cloud-providers/azure"
	"k8s.io/legacy-cloud-providers/azure/clients/fileclient"
)

func init() {
	framework.RegisterProvider("azure", newProvider)
}

func newProvider() (framework.ProviderInterface, error) {
	if framework.TestContext.CloudConfig.ConfigFile == "" {
		return nil, fmt.Errorf("config-file must be specified for Azure")
	}
	config, err := os.Open(framework.TestContext.CloudConfig.ConfigFile)
	if err != nil {
		framework.Logf("Couldn't open cloud provider configuration %s: %#v",
			framework.TestContext.CloudConfig.ConfigFile, err)
	} else {
		defer config.Close()
	}
	azureCloud, err := azure.NewCloud(config)
	if err != nil {
		return nil, err
	}
	return &Provider{
		azureCloud: azureCloud.(*azure.Cloud),
	}, nil
}

// Provider is a structure to handle Azure clouds for e2e testing
type Provider struct {
	framework.NullProvider

	azureCloud *azure.Cloud
}

// DeleteNode deletes a node which is specified as the argument
func (p *Provider) DeleteNode(node *v1.Node) error {
	return errors.New("not implemented yet")
}

// CreatePD creates a persistent volume
func (p *Provider) CreatePD(zone string) (string, error) {
	pdName := fmt.Sprintf("%s-%s", framework.TestContext.Prefix, string(uuid.NewUUID()))

	volumeOptions := &azure.ManagedDiskOptions{
		DiskName:           pdName,
		StorageAccountType: compute.StandardLRS,
		ResourceGroup:      "",
		PVCName:            pdName,
		SizeGB:             1,
		Tags:               nil,
		DiskIOPSReadWrite:  "",
		DiskMBpsReadWrite:  "",
	}

	// do not use blank zone definition
	if len(zone) > 0 {
		volumeOptions.AvailabilityZone = zone
	}
	return p.azureCloud.CreateManagedDisk(volumeOptions)
}

// CreateShare creates a share and return its account name and key.
func (p *Provider) CreateShare() (string, string, string, error) {
	accountOptions := &azure.AccountOptions{
		Name:                      "",
		Type:                      string(compute.StandardLRS),
		ResourceGroup:             p.azureCloud.ResourceGroup,
		Location:                  p.azureCloud.GetLocation(),
		EnableHTTPSTrafficOnly:    true,
		Tags:                      nil,
		VirtualNetworkResourceIDs: nil,
	}

	shareOptions := &fileclient.ShareOptions{
		Name:       fmt.Sprintf("%s-%s", framework.TestContext.Prefix, string(uuid.NewUUID())),
		RequestGiB: 1,
	}

	accountName, accountKey, err := p.azureCloud.CreateFileShare(accountOptions, shareOptions)
	if err != nil {
		return "", "", "", err
	}

	return accountName, accountKey, shareOptions.Name, nil
}

func (p *Provider) DeleteShare(accountName, shareName string) error {
	err := p.azureCloud.DeleteFileShare(p.azureCloud.ResourceGroup, accountName, shareName)
	if err != nil {
		framework.Logf("failed to delete Azure File share %q: %v", shareName, err)
	}
	return err
}

// DeletePD deletes a persistent volume
func (p *Provider) DeletePD(pdName string) error {
	if err := p.azureCloud.DeleteManagedDisk(pdName); err != nil {
		framework.Logf("failed to delete Azure volume %q: %v", pdName, err)
		return err
	}
	return nil
}

// EnableAndDisableInternalLB returns functions for both enabling and disabling internal Load Balancer
func (p *Provider) EnableAndDisableInternalLB() (enable, disable func(svc *v1.Service)) {
	enable = func(svc *v1.Service) {
		svc.ObjectMeta.Annotations = map[string]string{azure.ServiceAnnotationLoadBalancerInternal: "true"}
	}
	disable = func(svc *v1.Service) {
		svc.ObjectMeta.Annotations = map[string]string{azure.ServiceAnnotationLoadBalancerInternal: "false"}
	}
	return
}
