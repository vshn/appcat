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

type Sidecars map[string]sidecar

type sidecar struct {
	Limits struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"limits"`
	Requests struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"requests"`
}

func GetAllSideCarsResources(s *Sidecars) (Resources, error) {

	rTot := Resources{}

	for sidecar := range *s {
		r, err := s.convertSidecarToResource(sidecar)
		if err != nil {
			return Resources{}, err
		}
		rTot.AddResources(r)
	}

	return rTot, nil
}
func FetchSidecarsFromCluster(ctx context.Context, c client.Client, name string) (*Sidecars, error) {
	s := &Sidecars{}
	cm := &corev1.ConfigMap{}

	ns := viper.GetString("PLANS_NAMESPACE")
	key := client.ObjectKey{Name: name, Namespace: ns}
	err := c.Get(ctx, key, cm)

	if err != nil {
		return &Sidecars{}, err
	}

	err = json.Unmarshal([]byte(cm.Data["sideCars"]), s)
	if err != nil {
		return &Sidecars{}, err
	}

	return s, nil
}

func FetchSidecarsFromConfig(ctx context.Context, svc *runtime.ServiceRuntime) (*Sidecars, error) {
	s := &Sidecars{}

	err := json.Unmarshal([]byte(svc.Config.Data["sideCars"]), s)
	if err != nil {
		return &Sidecars{}, err
	}

	return s, nil
}

func FetchInitContainersFromConfig(ctx context.Context, svc *runtime.ServiceRuntime) (*Sidecars, error) {
	s := &Sidecars{}

	err := json.Unmarshal([]byte(svc.Config.Data["initContainers"]), s)
	if err != nil {
		return &Sidecars{}, err
	}

	return s, nil
}

// FetchSidecarFromCluster will fetch the specified sidecar from the current PLANS_NAMESPACE namespace and parse it into Resources.
// By default PLANS_NAMESPACE should be the same namespace where the controller pod is running.
func FetchSidecarFromCluster(ctx context.Context, c client.Client, name, sidecar string) (Resources, error) {
	s, err := FetchSidecarsFromCluster(ctx, c, name)
	if err != nil {
		return Resources{}, err
	}

	r, err := s.convertSidecarToResource(sidecar)

	return r, err
}

func (s Sidecars) convertSidecarToResource(sidecar string) (Resources, error) {
	r := Resources{}
	var err error

	sideCar := s[sidecar]

	r.CPURequests, err = resource.ParseQuantity(sideCar.Requests.CPU)
	if err != nil {
		return Resources{}, err
	}

	r.CPULimits, err = resource.ParseQuantity(sideCar.Limits.CPU)
	if err != nil {
		return Resources{}, err
	}

	r.MemoryRequests, err = resource.ParseQuantity(sideCar.Requests.Memory)
	if err != nil {
		return Resources{}, err
	}

	r.MemoryLimits, err = resource.ParseQuantity(sideCar.Limits.Memory)
	if err != nil {
		return Resources{}, err
	}

	return r, nil
}
