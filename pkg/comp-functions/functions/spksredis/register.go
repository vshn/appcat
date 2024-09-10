package spksredis

import (
	"github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("redis-k8s", runtime.Service[*v1alpha1.CompositeRedisInstance]{
		Steps: []runtime.Step[*v1alpha1.CompositeRedisInstance]{
			{
				Name:    "resizePVC",
				Execute: ResizeSpksPVCs,
			},
		},
	})
}
