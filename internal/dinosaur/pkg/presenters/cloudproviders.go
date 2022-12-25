package presenters

import (
	"github.com/stackrox/acs-fleet-manager/pkg/api/public"
	"github.com/stackrox/acs-fleet-manager/pkg/api"
)

// PresentCloudProvider ...
func PresentCloudProvider(cloudProvider *api.CloudProvider) public.CloudProvider {
	reference := PresentReference(cloudProvider.ID, cloudProvider)
	return public.CloudProvider{
		Id:          reference.Id,
		Kind:        reference.Kind,
		Name:        cloudProvider.Name,
		DisplayName: cloudProvider.DisplayName,
		Enabled:     cloudProvider.Enabled,
	}
}

// PresentCloudRegion ...
func PresentCloudRegion(cloudRegion *api.CloudRegion) public.CloudRegion {
	reference := PresentReference(cloudRegion.ID, cloudRegion)
	return public.CloudRegion{
		Id:                     reference.Id,
		Kind:                   reference.Kind,
		DisplayName:            cloudRegion.DisplayName,
		Enabled:                cloudRegion.Enabled,
		SupportedInstanceTypes: cloudRegion.SupportedInstanceTypes,
	}
}
