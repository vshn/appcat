package common

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestResources(t *testing.T) {

	ctx := context.Background()
	svc := commontest.LoadRuntimeFromFile(t, "plans.yaml")
	planResources, err := utils.FetchPlansFromConfig(ctx, svc, svc.Config.Data["defaultPlan"])
	assert.NoError(t, err)

	tests := []struct {
		name           string
		claimResources vshnv1.VSHNSizeSpec
		expResult      Resources
	}{
		{
			name:           "GivenDefaultPlanNoResources_ThenRequestsEqualLimits",
			claimResources: vshnv1.VSHNSizeSpec{},
			expResult: Resources{
				ReqMem: planResources.MemoryLimits,
				ReqCPU: planResources.CPULimits,
				Mem:    planResources.MemoryLimits,
				CPU:    planResources.CPULimits,
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenCustomLimitsOnly_ThenRequestsEqualLimits_QoSGuaranteed",
			claimResources: vshnv1.VSHNSizeSpec{
				Memory: "2Gi",
				CPU:    "500m",
			},
			expResult: Resources{
				ReqMem: resource.MustParse("2Gi"),
				ReqCPU: resource.MustParse("500m"),
				Mem:    resource.MustParse("2Gi"),
				CPU:    resource.MustParse("500m"),
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenCustomLimitsLowerThanPlan_ThenRequestsEqualLimits_QoSGuaranteed",
			claimResources: vshnv1.VSHNSizeSpec{
				Memory: "100Mi",
				CPU:    "100m",
			},
			expResult: Resources{
				ReqMem: resource.MustParse("100Mi"),
				ReqCPU: resource.MustParse("100m"),
				Mem:    resource.MustParse("100Mi"),
				CPU:    resource.MustParse("100m"),
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenExplicitRequestsOnly_ThenRequestsAsSpecified_LimitsFromRequests",
			claimResources: vshnv1.VSHNSizeSpec{
				Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
					Memory: "2Gi",
					CPU:    "1",
				},
			},
			expResult: Resources{
				ReqMem: resource.MustParse("2Gi"),
				ReqCPU: resource.MustParse("1"),
				Mem:    resource.MustParse("2Gi"),
				CPU:    resource.MustParse("1"),
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenRequestsHigherThanLimits_ThenLimitsRaisedToMatchRequests",
			claimResources: vshnv1.VSHNSizeSpec{
				Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
					Memory: "2Gi",
					CPU:    "1",
				},
				Memory: "500Mi",
				CPU:    "500m",
			},
			expResult: Resources{
				ReqMem: resource.MustParse("2Gi"),
				ReqCPU: resource.MustParse("1"),
				Mem:    resource.MustParse("2Gi"),
				CPU:    resource.MustParse("1"),
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenLimitsAndRequestsBothSet_ThenSeparateRequestsAndLimits_QoSBurstable",
			claimResources: vshnv1.VSHNSizeSpec{
				Memory: "3Gi",
				CPU:    "800m",
				Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
					Memory: "1Gi",
					CPU:    "400m",
				},
			},
			expResult: Resources{
				ReqMem: resource.MustParse("1Gi"),
				ReqCPU: resource.MustParse("400m"),
				Mem:    resource.MustParse("3Gi"),
				CPU:    resource.MustParse("800m"),
				Disk:   planResources.Disk,
			},
		},
		{
			name: "GivenCustomDisk_ThenDiskOverridesPlan",
			claimResources: vshnv1.VSHNSizeSpec{
				Disk: "50Gi",
			},
			expResult: Resources{
				ReqMem: planResources.MemoryLimits,
				ReqCPU: planResources.CPULimits,
				Mem:    planResources.MemoryLimits,
				CPU:    planResources.CPULimits,
				Disk:   resource.MustParse("50Gi"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, errs := GetResources(&tt.claimResources, planResources)
			assert.NoError(t, errors.Join(errs...))

			assert.Equal(t, tt.expResult, res)
		})
	}
}
