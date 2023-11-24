package utils

import (
	"context"
	"encoding/json"

	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Plans map[string]plan

type plan struct {
	Size struct {
		CPU     string `json:"cpu"`
		Disk    string `json:"disk"`
		Enabled bool   `json:"enabled"`
		Memory  string `json:"memory"`
	} `json:"Size"`
}

// FetchPlansFromCluster will fetch the plans from the current PLANS_NAMESPACE namespace and parse them into Resources.
// By default PLANS_NAMESPACE should be the same namespace where the controller pod is running.
func FetchPlansFromCluster(ctx context.Context, c client.Client, name, plan string) (Resources, error) {
	p := &Plans{}
	cm := &corev1.ConfigMap{}

	ns := viper.GetString("PLANS_NAMESPACE")
	key := client.ObjectKey{Name: name, Namespace: ns}
	err := c.Get(ctx, key, cm)
	if err != nil {
		return Resources{}, err
	}

	err = json.Unmarshal([]byte(cm.Data["plans"]), p)
	if err != nil {
		return Resources{}, err
	}

	r, err := convertPlanToResource(plan, *p)

	return r, err
}

func FetchPlansFromConfig(ctx context.Context, svc *runtime.ServiceRuntime, plan string) (Resources, error) {
	p := &Plans{}

	err := json.Unmarshal([]byte(svc.Config.Data["plans"]), p)
	if err != nil {
		return Resources{}, err
	}

	r, err := convertPlanToResource(plan, *p)

	return r, err
}

func convertPlanToResource(plan string, p Plans) (Resources, error) {
	r := Resources{}
	var err error

	planSize := p[plan]

	r.CPURequests, err = resource.ParseQuantity(planSize.Size.CPU)
	if err != nil {
		return Resources{}, err
	}
	r.CPULimits = r.CPURequests

	r.MemoryRequests, err = resource.ParseQuantity(planSize.Size.Memory)
	if err != nil {
		return Resources{}, err
	}
	r.MemoryLimits = r.MemoryRequests

	r.Disk, err = resource.ParseQuantity(planSize.Size.Disk)
	if err != nil {
		return Resources{}, err
	}

	return r, nil
}
