package common

import (
	"context"
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
	resources, err := utils.FetchPlansFromConfig(ctx, svc, svc.Config.Data["defaultPlan"])
	assert.NoError(t, err)

	tests := []struct {
		name      string
		size      vshnv1.VSHNSizeSpec
		expResult Resources
	}{
		{
			name: "GivenDefaultPlanNoResources",
			size: vshnv1.VSHNSizeSpec{},
			expResult: Resources{
				ReqMem: resources.MemoryRequests,
				ReqCPU: resources.CPURequests,
				Mem:    resources.MemoryLimits,
				CPU:    resources.CPULimits,
				Disk:   resources.Disk,
			},
		},
		{
			name: "GivenDefaultPlanCustomLimitHigherThanPlan",
			size: vshnv1.VSHNSizeSpec{
				Memory: "2Gi",
				CPU:    "500m",
			},
			expResult: Resources{
				ReqMem: resources.MemoryRequests,
				ReqCPU: resources.CPURequests,
				Mem:    resource.MustParse("2Gi"),
				CPU:    resource.MustParse("500m"),
				Disk:   resources.Disk,
			},
		},
		{
			name: "GivenDefaultPlanCustomMemoryLimitLowerThanPlan",
			size: vshnv1.VSHNSizeSpec{
				Memory: "100Mi",
				CPU:    "100m",
			},
			expResult: Resources{
				ReqMem: resource.MustParse("100Mi"),
				ReqCPU: resource.MustParse("100m"),
				Mem:    resource.MustParse("100Mi"),
				CPU:    resource.MustParse("100m"),
				Disk:   resources.Disk,
			},
		},
		{
			name: "GivenDefaultPlanCustomMemoryRequestsHigherThanPlan",
			size: vshnv1.VSHNSizeSpec{
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
				Disk:   resources.Disk,
			},
		},
		{
			name: "GivenDefaultPlanCustomMemoryRequestsHigherThanLimit",
			size: vshnv1.VSHNSizeSpec{
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
				Disk:   resources.Disk,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := GetResources(&tt.size, resources)
			assert.NoError(t, err)

			assert.Equal(t, tt.expResult, res)
		})
	}
}
