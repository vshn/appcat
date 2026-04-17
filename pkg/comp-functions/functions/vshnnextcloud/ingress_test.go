package vshnnextcloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestAddIngressHTTPRoute(t *testing.T) {
	t.Run("GivenHTTPRouteMode_ExpectHTTPRouteAndGrant", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshnnextcloud/03_httproute.yaml")
		comp := &vshnv1.VSHNNextcloud{}
		err := svc.GetObservedComposite(comp)
		assert.NoError(t, err)
		comp.Spec.Parameters.Service.FQDN = []string{"nc.example.com", "nc2.example.com"}

		result := AddIngress(context.Background(), comp, svc)
		assert.Nil(t, result)

		allDesired := svc.GetAllDesired()
		foundRoute := false
		foundGrant := false
		for _, d := range allDesired {
			name := d.Resource.GetName()
			if name == comp.GetName()+"-httproute" {
				foundRoute = true
			}
			if name == comp.GetName()+"-httpgrant" {
				foundGrant = true
			}
		}
		assert.True(t, foundRoute, "expected HTTPRoute to be created")
		assert.True(t, foundGrant, "expected ReferenceGrant to be created")
	})
}
