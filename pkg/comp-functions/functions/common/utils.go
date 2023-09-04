package common

import (
	"context"
	"encoding/json"

	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/apimachinery/pkg/api/resource"
)

type plans map[string]plan

type plan struct {
	Size struct {
		CPU     string `json:"cpu"`
		Disk    string `json:"disk"`
		Enabled bool   `json:"enabled"`
		Memory  string `json:"memory"`
	} `json:"Size"`
}

func FetchPlansFromConfig(ctx context.Context, iof *runtime.Runtime, plan string) (quotas.Resources, error) {
	p := &plans{}

	err := json.Unmarshal([]byte(iof.Config.Data["plans"]), p)
	if err != nil {
		return quotas.Resources{}, err
	}

	r, err := convertPlanToResource(plan, *p)

	return r, err
}

func convertPlanToResource(plan string, p plans) (quotas.Resources, error) {
	r := quotas.Resources{}
	var err error

	planSize := p[plan]

	r.CPURequests, err = resource.ParseQuantity(planSize.Size.CPU)
	if err != nil {
		return quotas.Resources{}, err
	}
	r.CPULimits = r.CPURequests

	r.MemoryRequests, err = resource.ParseQuantity(planSize.Size.Memory)
	if err != nil {
		return quotas.Resources{}, err
	}
	r.MemoryLimits = r.MemoryRequests

	r.Disk, err = resource.ParseQuantity(planSize.Size.Disk)
	if err != nil {
		return quotas.Resources{}, err
	}

	return r, nil
}
