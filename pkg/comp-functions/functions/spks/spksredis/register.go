package spksredis

import (
	"github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/spks/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("redis-k8s", runtime.Service[*v1alpha1.CompositeRedisInstance]{
		Steps: []runtime.Step[*v1alpha1.CompositeRedisInstance]{
			{
				Name:    "haproxyMetrics",
				Execute: HaproxyMetrics,
			},
			{
				Name:    "resizePVC",
				Execute: ResizeSpksPVCs,
			},
			{
				Name:    "handleTLS",
				Execute: HandleTLS,
			},
			// This one should be the last step to call!
			{
				Name:    "fixConnectionDetailsSecret",
				Execute: common.IgnoreConnectionDetailFix[*v1alpha1.CompositeRedisInstance],
			},
		},
	})
}
